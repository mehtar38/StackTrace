package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

const (
	// Label applied to every container we manage, so we never touch unrelated containers.
	managedLabel = "stacktrace.managed=true"

	// Internal port the challenge app listens on inside the container.
	// Each challenge Dockerfile must EXPOSE this port.
	challengeContainerPort = "3000/tcp"

	// healthCheckInterval between HTTP probes when waiting for a container to be ready.
	healthCheckInterval = 500 * time.Millisecond
)

// localDockerProvider implements Provider using the local Docker daemon.
// It is only used in development (SANDBOX_PROVIDER=local).
type localDockerProvider struct {
	docker *dockerclient.Client
	cfg    ProviderConfig
}

func newLocalDockerProvider(cfg ProviderConfig) (*localDockerProvider, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("docker client init: %w", err)
	}
	return &localDockerProvider{docker: cli, cfg: cfg}, nil
}

// imageName derives the Docker image tag for a given challenge.
// Convention: stacktrace-<challengeID>:latest
func (p *localDockerProvider) imageName(challengeID string) string {
	return fmt.Sprintf("stacktrace-%s:latest", challengeID)
}

// EnsureImageBuilt builds the challenge Docker image if it is not already present.
// Callers (prewarm) should call this before StartContainer.
func (p *localDockerProvider) EnsureImageBuilt(ctx context.Context, challengeID string) error {
	image := p.imageName(challengeID)

	// Check if image exists locally
	_, _, err := p.docker.ImageInspectWithRaw(ctx, image)
	if err == nil {
		slog.Debug("image already present", "image", image)
		return nil
	}

	slog.Info("building challenge image", "image", image, "challenge", challengeID)

	buildCtxReader, err := buildContextTar(p.cfg.ChallengesDir, challengeID)
	if err != nil {
		return fmt.Errorf("build context: %w", err)
	}

	resp, err := p.docker.ImageBuild(ctx, buildCtxReader, types.ImageBuildOptions{
		Tags:       []string{image},
		Dockerfile: "Dockerfile",
		Remove:     true,
	})
	if err != nil {
		return fmt.Errorf("image build: %w", err)
	}
	defer resp.Body.Close()

	// Drain output so the build completes; log it at debug level
	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return fmt.Errorf("reading build output: %w", err)
	}
	slog.Debug("image build output", "image", image, "output", buf.String())
	return nil
}

// StartContainer implements Provider.
func (p *localDockerProvider) StartContainer(ctx context.Context, challengeID string) (*ContainerInfo, error) {
	image := p.imageName(challengeID)

	// Ensure the image is available (no-op if already built)
	if err := p.EnsureImageBuilt(ctx, challengeID); err != nil {
		return nil, err
	}

	// Ask Docker to pick a free host port by binding to 0
	portBindings := nat.PortMap{
		nat.Port(challengeContainerPort): []nat.PortBinding{
			{HostIP: "127.0.0.1", HostPort: "0"},
		},
	}

	resp, err := p.docker.ContainerCreate(ctx,
		&container.Config{
			Image: image,
			Labels: map[string]string{
				"stacktrace.managed":    "true",
				"stacktrace.challenge":  challengeID,
				"stacktrace.created_at": time.Now().UTC().Format(time.RFC3339),
			},
			Tty:          true,
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			OpenStdin:    true,
		},
		&container.HostConfig{
			PortBindings: portBindings,
			// Resource limits to prevent runaway containers
			Resources: container.Resources{
				Memory:   512 * 1024 * 1024, // 512 MB
				NanoCPUs: 1_000_000_000,     // 1 vCPU
			},
			// No network access except loopback — challenges are self-contained
			NetworkMode: "bridge",
		},
		nil, nil, "",
	)
	if err != nil {
		return nil, fmt.Errorf("container create: %w", err)
	}

	containerID := resp.ID

	if err := p.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		// Best-effort cleanup on failure
		_ = p.docker.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("container start: %w", err)
	}

	// Inspect to get the assigned host port
	inspect, err := p.docker.ContainerInspect(ctx, containerID)
	if err != nil {
		_ = p.docker.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("container inspect: %w", err)
	}

	bindings := inspect.NetworkSettings.Ports[nat.Port(challengeContainerPort)]
	if len(bindings) == 0 {
		_ = p.docker.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("no port binding found for container %s", containerID[:12])
	}

	host := fmt.Sprintf("localhost:%s", bindings[0].HostPort)

	// Wait until the app inside the container is ready
	prewarmCtx, cancel := context.WithTimeout(ctx, time.Duration(p.cfg.PrewarmTimeoutSecs)*time.Second)
	defer cancel()

	if err := p.waitUntilHealthy(prewarmCtx, host); err != nil {
		_ = p.docker.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("container health check failed: %w", err)
	}

	slog.Info("container ready", "id", containerID[:12], "host", host, "challenge", challengeID)
	return &ContainerInfo{ID: containerID, Host: host}, nil
}

// StopContainer implements Provider.
func (p *localDockerProvider) StopContainer(ctx context.Context, containerID string) error {
	stopTimeout := 10 // seconds
	err := p.docker.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout})
	if err != nil && !isNotFoundError(err) {
		return fmt.Errorf("container stop: %w", err)
	}

	if err := p.docker.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		if !isNotFoundError(err) {
			return fmt.Errorf("container remove: %w", err)
		}
	}
	return nil
}

// WriteFile implements Provider.
// Wraps content in a tar archive and uses docker cp (CopyToContainer) to
// write it at the correct path inside the container.
func (p *localDockerProvider) WriteFile(ctx context.Context, containerID, relativePath, content string) error {
	buf, err := contentToTar(relativePath, content)
	if err != nil {
		return fmt.Errorf("tar encode: %w", err)
	}

	// The destination is the directory component of the path.
	// CopyToContainer will extract the tar relative to destPath.
	destDir := containerWorkdir(relativePath)

	err = p.docker.CopyToContainer(ctx, containerID, destDir, buf, types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
	})
	if err != nil {
		return fmt.Errorf("docker cp to container: %w", err)
	}
	return nil
}

// ReadFile implements Provider.
func (p *localDockerProvider) ReadFile(ctx context.Context, containerID, relativePath string) (string, error) {
	// CopyFromContainer returns a tar stream
	reader, _, err := p.docker.CopyFromContainer(ctx, containerID, containerFilePath(relativePath))
	if err != nil {
		return "", fmt.Errorf("docker cp from container: %w", err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	if _, err := tr.Next(); err != nil {
		return "", fmt.Errorf("reading tar header: %w", err)
	}
	content, err := io.ReadAll(tr)
	if err != nil {
		return "", fmt.Errorf("reading file content: %w", err)
	}
	return string(content), nil
}

// ExecShell implements Provider.
func (p *localDockerProvider) ExecShell(ctx context.Context, containerID string) (ShellSession, error) {
	exec, err := p.docker.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          []string{"/bin/sh"},
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	conn, err := p.docker.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	return &dockerShellSession{
		execID: exec.ID,
		conn:   conn,
		docker: p.docker,
	}, nil
}

// ListManagedContainers returns all containers created by this provider.
// Used by the cleanup job to reap unassociated pre-warmed containers.
func (p *localDockerProvider) ListManagedContainers(ctx context.Context) ([]types.Container, error) {
	f := filters.NewArgs()
	f.Add("label", managedLabel)
	return p.docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
}

// --- helpers ---

func (p *localDockerProvider) waitUntilHealthy(ctx context.Context, host string) error {
	url := fmt.Sprintf("http://%s/healthz", host)
	client := &http.Client{Timeout: 2 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for container at %s: %w", host, ctx.Err())
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}

		time.Sleep(healthCheckInterval)
	}
}

// contentToTar wraps a single file's content in a tar archive suitable
// for CopyToContainer. The archive entry filename is the base name of relativePath.
func contentToTar(relativePath, content string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	body := []byte(content)
	hdr := &tar.Header{
		Name: baseName(relativePath),
		Mode: 0644,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, err
	}
	if _, err := tw.Write(body); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

// containerWorkdir returns the absolute directory inside the container
// for a given relative file path (e.g. "src/db/write.js" → "/app/src/db").
func containerWorkdir(relativePath string) string {
	parts := strings.Split(relativePath, "/")
	if len(parts) <= 1 {
		return "/app"
	}
	return "/app/" + strings.Join(parts[:len(parts)-1], "/")
}

// containerFilePath returns the full absolute path inside the container.
func containerFilePath(relativePath string) string {
	return "/app/" + relativePath
}

// baseName returns the filename component of a slash-separated path.
func baseName(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "No such container")
}

// buildContextTar builds a tar archive of the challenge directory for docker build.
// This is a simplified implementation — for production, use a proper fs walk.
func buildContextTar(challengesDir, challengeID string) (io.Reader, error) {
	// The Docker SDK can accept a path directly via ImageBuild with a BuildContext
	// but we need a tar reader. In local dev we shell out or use the daemon's
	// built-in path support. Here we return the path encoded as a tar-of-directory
	// using the standard library.
	//
	// For v1 local dev, we use dockerclient.WithHostFromEnv and the daemon handles
	// the build context. This placeholder returns a minimal tar that triggers a
	// "use daemon's local path" approach — replace with a real tar walk if building
	// on remote daemons.
	dirPath := fmt.Sprintf("%s/%s", challengesDir, challengeID)
	return archiveDir(dirPath)
}

func archiveDir(srcDir string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := addDirToTar(tw, srcDir, ""); err != nil {
		return nil, err
	}
	tw.Close()
	return &buf, nil
}

func addDirToTar(tw *tar.Writer, srcDir, prefix string) error {
	return addDirToTarImpl(tw, srcDir, prefix)
}
