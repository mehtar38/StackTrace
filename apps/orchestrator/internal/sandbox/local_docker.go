package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	// "github.com/docker/docker/api/types/build"
	"github.com/docker/docker/api/types/container"
	// "github.com/docker/docker/api/types/exec"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

const (
	managedLabel           = "stacktrace.managed=true"
	challengeContainerPort = "3000/tcp"
	healthCheckInterval    = 500 * time.Millisecond
)

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

func (p *localDockerProvider) imageName(challengeID string) string {
	return fmt.Sprintf("stacktrace-%s:latest", challengeID)
}

func (p *localDockerProvider) EnsureImageBuilt(ctx context.Context, challengeID string) error {
	image := p.imageName(challengeID)
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

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return fmt.Errorf("reading build output: %w", err)
	}
	slog.Debug("image build output", "image", image, "output", buf.String())
	return nil
}

func (p *localDockerProvider) StartContainer(ctx context.Context, challengeID string) (*ContainerInfo, error) {
	image := p.imageName(challengeID)

	if err := p.EnsureImageBuilt(ctx, challengeID); err != nil {
		return nil, err
	}

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
			Resources: container.Resources{
				Memory:   512 * 1024 * 1024,
				NanoCPUs: 1_000_000_000,
			},
			NetworkMode: "bridge",
		},
		nil, nil, "",
	)
	if err != nil {
		return nil, fmt.Errorf("container create: %w", err)
	}

	containerID := resp.ID

	if err := p.docker.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		_ = p.docker.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("container start: %w", err)
	}

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

	prewarmCtx, cancel := context.WithTimeout(ctx, time.Duration(p.cfg.PrewarmTimeoutSecs)*time.Second)
	defer cancel()

	if err := waitUntilHealthyTCP(prewarmCtx, host); err != nil {
		_ = p.docker.ContainerRemove(context.Background(), containerID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("container health check failed: %w", err)
	}

	slog.Info("container ready", "id", containerID[:12], "host", host, "challenge", challengeID)
	return &ContainerInfo{ID: containerID, Host: host}, nil
}

func (p *localDockerProvider) StopContainer(ctx context.Context, containerID string) error {
	stopTimeout := 10
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

func (p *localDockerProvider) WriteFile(ctx context.Context, containerID, relativePath, content string) error {
	buf, err := contentToTar(relativePath, content)
	if err != nil {
		return fmt.Errorf("tar encode: %w", err)
	}
	destDir := containerWorkdir(relativePath)
	err = p.docker.CopyToContainer(ctx, containerID, destDir, buf, types.CopyToContainerOptions{
		AllowOverwriteDirWithFile: false,
	})
	if err != nil {
		return fmt.Errorf("docker cp to container: %w", err)
	}
	return nil
}

func (p *localDockerProvider) ReadFile(ctx context.Context, containerID, relativePath string) (string, error) {
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

func (p *localDockerProvider) ExecShell(ctx context.Context, containerID string) (ShellSession, error) {
	execResp, err := p.docker.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          []string{"/bin/sh"},
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	conn, err := p.docker.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{Tty: true})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	return &dockerShellSession{
		execID: execResp.ID,
		conn:   conn,
		docker: p.docker,
	}, nil
}

func (p *localDockerProvider) ListManagedContainers(ctx context.Context) ([]types.Container, error) {
	f := filters.NewArgs()
	f.Add("label", managedLabel)
	return p.docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
}

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

func containerWorkdir(relativePath string) string {
	parts := strings.Split(relativePath, "/")
	if len(parts) <= 1 {
		return "/app"
	}
	return "/app/" + strings.Join(parts[:len(parts)-1], "/")
}

func containerFilePath(relativePath string) string {
	return "/app/" + relativePath
}

func baseName(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "No such container")
}

func buildContextTar(challengesDir, challengeID string) (io.Reader, error) {
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

func (p *localDockerProvider) ExecCommand(ctx context.Context, containerID string, cmd []string) (string, error) {
	execResp, err := p.docker.ContainerExecCreate(ctx, containerID, types.ExecConfig{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          false,
	})
	if err != nil {
		return "", fmt.Errorf("exec create: %w", err)
	}

	conn, err := p.docker.ContainerExecAttach(ctx, execResp.ID, types.ExecStartCheck{Tty: false})
	if err != nil {
		return "", fmt.Errorf("exec attach: %w", err)
	}
	defer conn.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := stdcopy.StdCopy(&outBuf, &errBuf, conn.Reader); err != nil {
		return "", fmt.Errorf("exec read: %w", err)
	}

	return outBuf.String() + errBuf.String(), nil
}
