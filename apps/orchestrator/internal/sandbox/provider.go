// Package sandbox defines the SandboxProvider interface and its factory.
// All container lifecycle operations go through this interface — no caller
// ever imports a concrete provider directly.
//
// Local development uses LocalDockerProvider (talks to the local Docker daemon).
// Production uses AzureContainerAppsProvider (calls the ACA REST API).
// The active provider is selected by the SANDBOX_PROVIDER env var.
package sandbox

import (
	"context"
	"fmt"
)

// ContainerInfo describes a running sandbox container.
type ContainerInfo struct {
	// ID is the provider-specific container identifier.
	// Docker: container ID (64-char hex)
	// ACA: container app revision name
	ID string

	// Host is the address used to connect to the container's exposed port.
	// Docker local: "localhost:PORT"
	// ACA: "https://FQDN" (TLS termination handled by ACA)
	Host string
}

// ProviderConfig holds provider-agnostic configuration passed from the
// environment. Individual providers pick what they need.
type ProviderConfig struct {
	// ChallengesDir is the absolute path to the /challenges directory,
	// used by LocalDockerProvider to locate Dockerfiles and build images.
	ChallengesDir string

	// PrewarmTimeoutSecs is the maximum seconds to wait for a pre-warmed
	// container to become healthy before giving up.
	PrewarmTimeoutSecs int

	// ContainerTimeoutSecs is the hard wall-clock limit for any container.
	// The session manager enforces this independently; the provider uses it
	// as a Docker stop-timeout hint.
	ContainerTimeoutSecs int
}

// Provider is the interface every sandbox backend must implement.
// All methods must be safe to call concurrently.
type Provider interface {
	// StartContainer spins up a sandbox container for the given challenge.
	// It blocks until the container is healthy (HTTP /healthz or TCP probe).
	// Returns a ContainerInfo on success.
	StartContainer(ctx context.Context, challengeID string) (*ContainerInfo, error)

	// StopContainer stops and removes the container identified by containerID.
	// It is safe to call on an already-stopped container.
	StopContainer(ctx context.Context, containerID string) error

	// WriteFile writes the full content of a single file into the container
	// at the given relative path. Equivalent to `docker cp` for the local
	// provider. Used by the file-sync endpoint (Monaco → container).
	WriteFile(ctx context.Context, containerID, relativePath, content string) error

	// ReadFile reads the full content of a single file from the container.
	// Used to hydrate Monaco on session resume.
	ReadFile(ctx context.Context, containerID, relativePath string) (string, error)

	// ExecShell attaches an interactive PTY session to the container and
	// returns a pair of channels for reading output and writing input.
	// The returned cleanup func must be called when the WebSocket closes.
	ExecShell(ctx context.Context, containerID string) (ShellSession, error)
}

// ShellSession represents an active PTY session inside a container.
type ShellSession interface {
	// Write sends bytes to the container's stdin.
	Write(p []byte) (int, error)

	// Read reads bytes from the container's stdout/stderr.
	Read(p []byte) (int, error)

	// Resize resizes the PTY to the given dimensions.
	Resize(rows, cols uint16) error

	// Close terminates the exec session and releases resources.
	Close() error
}

// NewProvider constructs the provider selected by providerName.
// Valid values: "local", "azure".
func NewProvider(providerName string, cfg ProviderConfig) (Provider, error) {
	switch providerName {
	case "local":
		return newLocalDockerProvider(cfg)
	case "azure":
		return newAzureContainerAppsProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown SANDBOX_PROVIDER %q: must be \"local\" or \"azure\"", providerName)
	}
}
