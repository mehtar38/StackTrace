package sandbox

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// dockerShellSession implements ShellSession for a Docker exec PTY.
type dockerShellSession struct {
	execID string
	conn   types.HijackedResponse
	docker *dockerclient.Client
}

// Write sends bytes to the container's stdin.
func (s *dockerShellSession) Write(p []byte) (int, error) {
	return s.conn.Conn.Write(p)
}

// Read reads bytes from the container's stdout/stderr stream.
// With TTY=true the stream is raw and can be read directly.
func (s *dockerShellSession) Read(p []byte) (int, error) {
	return s.conn.Reader.Read(p)
}

// ReadDemuxed demultiplexes the Docker stream (for TTY=false mode).
// Not used in the current implementation (TTY=true), exposed for completeness.
func (s *dockerShellSession) ReadDemuxed() (stdout, stderr []byte, err error) {
	var outBuf, errBuf bytes.Buffer
	_, err = stdcopy.StdCopy(&outBuf, &errBuf, s.conn.Reader)
	return outBuf.Bytes(), errBuf.Bytes(), err
}

// Resize resizes the PTY to the given terminal dimensions.
func (s *dockerShellSession) Resize(rows, cols uint16) error {
	return s.docker.ContainerExecResize(context.Background(), s.execID, container.ResizeOptions{
		Height: uint(rows),
		Width:  uint(cols),
	})
}

// Close terminates the exec session and releases all resources.
func (s *dockerShellSession) Close() error {
	s.conn.Close()
	return nil
}

var _ ShellSession = (*dockerShellSession)(nil)

// awsExecShellSession is a placeholder for the ECS Exec implementation.
// Real implementation will wrap an SSM Session Manager plugin subprocess,
// since the AWS SDK does not expose a native bidirectional session client.
type awsExecShellSession struct{}

func (s *awsExecShellSession) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("aws ecs exec shell session not yet implemented")
}
func (s *awsExecShellSession) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("aws ecs exec shell session not yet implemented")
}
func (s *awsExecShellSession) Resize(rows, cols uint16) error {
	return fmt.Errorf("aws ecs exec shell session not yet implemented")
}
func (s *awsExecShellSession) Close() error { return nil }

var _ ShellSession = (*awsExecShellSession)(nil)
