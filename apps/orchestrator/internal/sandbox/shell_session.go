package sandbox

import (
	"bytes"
	"context"
	"fmt"

	"github.com/docker/docker/api/types"
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

// Read reads bytes from the container's multiplexed stdout/stderr stream.
// Docker multiplexes stdout/stderr in TTY=false mode; with TTY=true (which
// we use), the stream is raw and can be read directly.
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
	return s.docker.ContainerExecResize(context.Background(), s.execID, types.ResizeOptions{
		Height: uint(rows),
		Width:  uint(cols),
	})
}

// Close terminates the exec session and releases all resources.
func (s *dockerShellSession) Close() error {
	s.conn.Close()
	// Exec sessions self-terminate when their process exits; no explicit kill needed.
	// We close the hijacked connection to release the net.Conn.
	return nil
}

// Ensure compile-time interface satisfaction
var _ ShellSession = (*dockerShellSession)(nil)

// azureShellSession is a placeholder for the ACA implementation.
// ACA exposes a WebSocket-based exec endpoint; this will proxy through that.
type azureShellSession struct{}

func (s *azureShellSession) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("azure shell session not yet implemented")
}
func (s *azureShellSession) Read(p []byte) (int, error) {
	return 0, fmt.Errorf("azure shell session not yet implemented")
}
func (s *azureShellSession) Resize(rows, cols uint16) error {
	return fmt.Errorf("azure shell session not yet implemented")
}
func (s *azureShellSession) Close() error { return nil }

var _ ShellSession = (*azureShellSession)(nil)
