package sandbox

import (
	"context"
	"fmt"
)

// awsECSProvider implements Provider using AWS ECS Fargate.
// This provider is used in production (SANDBOX_PROVIDER=aws).
//
// Implementation status: STUB — not yet implemented.
// LocalDockerProvider remains the dev-time provider; this is filled in next.
//
// Architecture notes for implementer:
//   - Use the AWS SDK for Go v2 (github.com/aws/aws-sdk-go-v2/service/ecs)
//   - Each session = one ECS RunTask call on a Fargate launch type
//   - Tasks are ephemeral: started on-demand, stopped via StopTask
//   - Container images live in ECR, not a local Docker daemon
//   - Terminal + ExecCommand: no Docker exec equivalent exists on Fargate.
//     Use ECS Exec, which tunnels through AWS Systems Manager (SSM) Session
//     Manager. This requires:
//     1. enableExecuteCommand: true on the task definition
//     2. An SSM-permitted IAM task role attached to the task
//     3. The aws-sdk-go-v2 ecs.ExecuteCommand API, or shelling out to the
//     `aws ecs execute-command` CLI and proxying its stdio, since the
//     Go SDK does not yet expose a native bidirectional session-manager
//     client — most production Go implementations wrap the SSM
//     Session Manager plugin binary as a subprocess.
//   - WriteFile/ReadFile: no `docker cp` equivalent. Implement via ExecCommand
//     running `cat > path <<EOF ... EOF` for writes and `cat path` for reads.
//   - Auth: AWS SDK default credential chain — IAM task role when running on
//     ECS itself, local AWS CLI profile when running locally for testing.
type awsECSProvider struct {
	cfg ProviderConfig
}

func newAWSECSProvider(cfg ProviderConfig) (*awsECSProvider, error) {
	// TODO: initialise AWS SDK ECS client
	// - ecsClient := ecs.NewFromConfig(awsCfg)
	return &awsECSProvider{cfg: cfg}, nil
}

func (p *awsECSProvider) StartContainer(ctx context.Context, challengeID string) (*ContainerInfo, error) {
	return nil, fmt.Errorf("AWSECSProvider.StartContainer: not yet implemented")
}

func (p *awsECSProvider) StopContainer(ctx context.Context, containerID string) error {
	return fmt.Errorf("AWSECSProvider.StopContainer: not yet implemented")
}

func (p *awsECSProvider) WriteFile(ctx context.Context, containerID, relativePath, content string) error {
	return fmt.Errorf("AWSECSProvider.WriteFile: not yet implemented")
}

func (p *awsECSProvider) ReadFile(ctx context.Context, containerID, relativePath string) (string, error) {
	return "", fmt.Errorf("AWSECSProvider.ReadFile: not yet implemented")
}

func (p *awsECSProvider) ExecShell(ctx context.Context, containerID string) (ShellSession, error) {
	return nil, fmt.Errorf("AWSECSProvider.ExecShell: not yet implemented")
}

func (p *awsECSProvider) ExecCommand(ctx context.Context, containerID string, cmd []string) (string, error) {
	return "", fmt.Errorf("AWSECSProvider.ExecCommand: not yet implemented")
}

var _ Provider = (*awsECSProvider)(nil)
