package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/creack/pty"
)

const maxExecRetries = 3
const ssmCorruptionMarker = "Cannot perform start session"

func (p *awsECSProvider) runExecuteCommandWithRetry(ctx context.Context, taskArn string, cmd []string) (string, error) {
	var lastErr error
	for attempt := 1; attempt <= maxExecRetries; attempt++ {
		if attempt > 1 {
			time.Sleep(3 * time.Second)
		}
		output, err := p.runExecuteCommand(ctx, taskArn, cmd)
		if err != nil {
			lastErr = err
			continue
		}
		if strings.Contains(output, ssmCorruptionMarker) {
			lastErr = fmt.Errorf("ssm stream corrupted (attempt %d/%d)", attempt, maxExecRetries)
			continue
		}
		return output, nil
	}
	return "", fmt.Errorf("exec command failed after %d attempts: %w", maxExecRetries, lastErr)
}

// ECS Exec has no native Go SDK stream — ecs.ExecuteCommand only returns connection metadata (a WebSocket URL + token pair).
// actual byte stream handled by SSM Session Manager plugin, a separate AWS-distributed binary in the orchestrator's own container image

// runExecuteCommand calls ecs.ExecuteCommand and shells out to the session-manager-plugin to actually run cmd inside the task,
// returning its combined stdout. Used for one-shot operations (find, cat, file writes)

func (p *awsECSProvider) runExecuteCommand(ctx context.Context, taskArn string, cmd []string) (string, error) {
	commandStr := strings.Join(cmd, " ")

	resp, err := p.ecsClient.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(p.cfg.ECSClusterName),
		Task:        aws.String(taskArn),
		Container:   aws.String("app"), // container name from the task definition
		Command:     aws.String(commandStr),
		Interactive: true, // ECS Exec requires this even for one-shot commands
	})
	if err != nil {
		return "", fmt.Errorf("ecs execute command: %w", err)
	}

	// The session details (stream URL, token, session ID) come back as a
	// JSON document that session-manager-plugin expects on argv, not stdin.
	sessionJSON, err := json.Marshal(resp.Session)
	if err != nil {
		return "", fmt.Errorf("marshal session: %w", err)
	}

	region := p.cfg.AWSConfig.Region

	// session-manager-plugin's takes the session JSON, region, operation name ("StartSession"), empty profile, ExecuteCommand
	// parameters JSON, and the SSM endpoint — mirroring exactly what the `aws ecs execute-command` CLI does internally.
	pluginCmd := exec.CommandContext(ctx, "session-manager-plugin",
		string(sessionJSON),
		region,
		"StartSession",
		"",
		fmt.Sprintf(`{"Target":"%s"}`, buildSSMTarget(taskArn)),
		fmt.Sprintf("https://ssm.%s.amazonaws.com", region),
	)

	var stdout, stderr bytes.Buffer
	pluginCmd.Stdout = &stdout
	pluginCmd.Stderr = &stderr

	if err := pluginCmd.Run(); err != nil {
		return "", fmt.Errorf("session-manager-plugin: %w (stderr: %s)", err, stderr.String())
	}

	return stripSSMBanner(stdout.String()), nil
}

func stripSSMBanner(output string) string {
	lines := strings.Split(output, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Starting session with SessionId") {
			continue
		}
		if strings.HasPrefix(trimmed, "Cannot perform start session") {
			continue
		}
		if strings.HasPrefix(trimmed, "Exiting session with sessionId") {
			continue
		}
		filtered = append(filtered, line)
	}
	return strings.Join(filtered, "\n")
}

// constructs SSM target identifier from a task ARN.
// Format: ecs:<cluster>_<task-id>_<runtime-id> — the runtime ID is the
// container's specific identifier within the task, which ECS Exec needs
// to address the right container if a task ever has more than one.
// For our single-container tasks, ECS accepts the simpler cluster_taskID form.
func buildSSMTarget(taskArn string) string {
	parts := strings.Split(taskArn, "/")
	taskID := parts[len(parts)-1]
	return taskID
}

// ExecCommand implements Provider. Runs a one-shot command inside the ECS
// task and returns combined output, used by the file tree listing handler.
func (p *awsECSProvider) ExecCommand(ctx context.Context, containerID string, cmd []string) (string, error) {
	return p.runExecuteCommandWithRetry(ctx, containerID, cmd)
}

// WriteFile implements Provider. No `docker cp` equivalent on Fargate, content written via shell heredoc redirection through ExecCommand
func (p *awsECSProvider) WriteFile(ctx context.Context, containerID, relativePath, content string) error {
	destPath := containerFilePath(relativePath)

	// heredoc with a randomized-enough delimiter to avoid collision with file content that happens to contain "EOF" on its
	// own line. 'EOF_STACKTRACE_WRITE' unlikely to appear in real source files.
	cmd := []string{
		"sh", "-c",
		fmt.Sprintf("cat > %s <<'EOF_STACKTRACE_WRITE'\n%s\nEOF_STACKTRACE_WRITE", destPath, content),
	}

	if _, err := p.runExecuteCommandWithRetry(ctx, containerID, cmd); err != nil {
		return fmt.Errorf("ecs exec write file: %w", err)
	}
	return nil
}

// ReadFile implements Provider. Reads via `cat` through ExecCommand.
func (p *awsECSProvider) ReadFile(ctx context.Context, containerID, relativePath string) (string, error) {
	destPath := containerFilePath(relativePath)

	output, err := p.runExecuteCommandWithRetry(ctx, containerID, []string{"cat", destPath})
	if err != nil {
		return "", fmt.Errorf("ecs exec read file: %w", err)
	}
	return output, nil
}

func (p *awsECSProvider) ExecShell(ctx context.Context, containerID string) (ShellSession, error) {
	resp, err := p.ecsClient.ExecuteCommand(ctx, &ecs.ExecuteCommandInput{
		Cluster:     aws.String(p.cfg.ECSClusterName),
		Task:        aws.String(containerID),
		Container:   aws.String("app"),
		Command:     aws.String("/bin/sh"),
		Interactive: true,
	})
	if err != nil {
		return nil, fmt.Errorf("ecs execute command (shell): %w", err)
	}

	sessionJSON, err := json.Marshal(resp.Session)
	if err != nil {
		return nil, fmt.Errorf("marshal session: %w", err)
	}

	region := p.cfg.AWSConfig.Region

	pluginCmd := exec.CommandContext(ctx, "session-manager-plugin",
		string(sessionJSON),
		region,
		"StartSession",
		"",
		fmt.Sprintf(`{"Target":"%s"}`, buildSSMTarget(containerID)),
		fmt.Sprintf("https://ssm.%s.amazonaws.com", region),
	)

	// Start the subprocess attached to a real PTY instead of plain pipes.
	// pty.Start allocates a pseudo-terminal pair and wires the subprocess's
	// stdin/stdout/stderr to the slave side, while returning the master side
	// (ptmx) as a single read/write file — this is what makes the plugin and
	// the remote shell correctly negotiate terminal capabilities.
	ptmx, err := pty.Start(pluginCmd)
	if err != nil {
		return nil, fmt.Errorf("start session-manager-plugin with pty: %w", err)
	}

	return &ecsExecShellSession{
		cmd:  pluginCmd,
		ptmx: ptmx,
	}, nil
}

// ecsExecShellSession implements ShellSession by wrapping a running
// session-manager-plugin subprocess. Resize is a no-op — the SSM plugin's
// PTY handling does not expose a resize control channel the way Docker's
// exec API does; terminal resize is best-effort/cosmetic on this provider.
type ecsExecShellSession struct {
	cmd  *exec.Cmd
	ptmx *os.File // single read/write handle to the PTY master side
}

func (s *ecsExecShellSession) Write(p []byte) (int, error) {
	return s.ptmx.Write(p)
}

func (s *ecsExecShellSession) Read(p []byte) (int, error) {
	return s.ptmx.Read(p)
}

func (s *ecsExecShellSession) Resize(rows, cols uint16) error {
	return pty.Setsize(s.ptmx, &pty.Winsize{Rows: rows, Cols: cols})
}

func (s *ecsExecShellSession) Close() error {
	_ = s.ptmx.Close()
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return s.cmd.Wait()
}

var _ ShellSession = (*ecsExecShellSession)(nil)
