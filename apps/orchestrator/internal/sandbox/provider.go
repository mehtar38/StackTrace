package sandbox

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
)

type ContainerInfo struct {
	ID   string
	Host string
}

type ProviderConfig struct {
	ChallengesDir        string
	PrewarmTimeoutSecs   int
	ContainerTimeoutSecs int

	// AWS ECS — only used by awsECSProvider, ignored by localDockerProvider.
	AWSConfig           aws.Config
	ECSClusterName      string
	ECSSubnetID         string
	ECSSecurityGroupID  string
	ECSTaskRoleArn      string
	ECSExecutionRoleArn string
	ECRRegistryURL      string
}

// Provider is the interface every sandbox backend must implement.
type Provider interface {
	StartContainer(ctx context.Context, challengeID string) (*ContainerInfo, error)
	StopContainer(ctx context.Context, containerID string) error
	WriteFile(ctx context.Context, containerID, relativePath, content string) error
	ReadFile(ctx context.Context, containerID, relativePath string) (string, error)
	ExecShell(ctx context.Context, containerID string) (ShellSession, error)
	// ExecCommand runs a one-shot command inside the container and returns
	// the combined stdout+stderr output as a string. Used for file tree listing.
	ExecCommand(ctx context.Context, containerID string, cmd []string) (string, error)
}

type ShellSession interface {
	Write(p []byte) (int, error)
	Read(p []byte) (int, error)
	Resize(rows, cols uint16) error
	Close() error
}

func NewProvider(providerName string, cfg ProviderConfig) (Provider, error) {
	switch providerName {
	case "local":
		return newLocalDockerProvider(cfg)
	case "aws":
		return newAWSECSProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown SANDBOX_PROVIDER %q: must be \"local\" or \"aws\"", providerName)
	}
}
