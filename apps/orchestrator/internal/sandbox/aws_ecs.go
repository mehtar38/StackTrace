package sandbox

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// awsECSProvider implements Provider using AWS ECS Fargate.
//
// Container identity: ContainerInfo.ID holds the ECS task ARN (not a Docker
// container ID). ContainerInfo.Host holds the task's PRIVATE IP within the
// VPC — the orchestrator itself runs in the same VPC/subnet, so it reaches
// challenge tasks directly over the private network. Challenge tasks never
// get a public IP and their security group only allows inbound from the
// orchestrator's own security group — end users never talk to a challenge
// container directly, only through the orchestrator.
type awsECSProvider struct {
	cfg       ProviderConfig
	ecsClient *ecs.Client
}

func newAWSECSProvider(cfg ProviderConfig) (*awsECSProvider, error) {
	return &awsECSProvider{
		cfg:       cfg,
		ecsClient: ecs.NewFromConfig(cfg.AWSConfig),
	}, nil
}

// taskDefinitionFamily derives the ECS task definition family name for a
// challenge. Convention matches the local provider's image naming:
// stacktrace-<challengeID>
func taskDefinitionFamily(challengeID string) string {
	return fmt.Sprintf("stacktrace-%s", challengeID)
}

// StartContainer implements Provider. Runs a new Fargate task from the
// challenge's pre-pushed task definition (images are built and pushed to ECR
// as a deploy-time step — the orchestrator never builds images itself).
func (p *awsECSProvider) StartContainer(ctx context.Context, challengeID string) (*ContainerInfo, error) {
	family := taskDefinitionFamily(challengeID)

	runResp, err := p.ecsClient.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:        aws.String(p.cfg.ECSClusterName),
		TaskDefinition: aws.String(family),
		LaunchType:     ecstypes.LaunchTypeFargate,
		Count:          aws.Int32(1),
		// ECS Exec requires this on every RunTask call — it cannot be
		// retrofitted onto an already-running task.
		EnableExecuteCommand: true,
		NetworkConfiguration: &ecstypes.NetworkConfiguration{
			AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
				Subnets:        []string{p.cfg.ECSSubnetID},
				SecurityGroups: []string{p.cfg.ECSSecurityGroupID},
				// DISABLED: challenge tasks never get a public IP. Only the
				// orchestrator's own service needs public reachability.
				// AssignPublicIp: ecstypes.AssignPublicIpDisabled,
				AssignPublicIp: ecstypes.AssignPublicIpEnabled,
			},
		},
		Tags: []ecstypes.Tag{
			{Key: aws.String("stacktrace.managed"), Value: aws.String("true")},
			{Key: aws.String("stacktrace.challenge"), Value: aws.String(challengeID)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ecs run task: %w", err)
	}
	if len(runResp.Tasks) == 0 {
		return nil, fmt.Errorf("ecs run task: no tasks returned (failures: %v)", runResp.Failures)
	}

	taskArn := aws.ToString(runResp.Tasks[0].TaskArn)

	// Wait for the task to reach RUNNING and resolve its private IP — both
	// happen together since the ENI (and therefore the IP) isn't attached
	// until the task actually starts.
	privateIP, err := p.waitForRunningAndResolveIP(ctx, taskArn)
	if err != nil {
		_ = p.StopContainer(context.Background(), taskArn)
		return nil, fmt.Errorf("wait for task running: %w", err)
	}

	slog.Info("waiting for exec agent to become ready", "task_arn", taskArn)
	if err := p.waitForExecAgentReady(ctx, taskArn); err != nil {
		_ = p.StopContainer(context.Background(), taskArn)
		return nil, fmt.Errorf("wait for exec agent: %w", err)
	}
	slog.Info("exec agent ready", "task_arn", taskArn)
	// time.Sleep(12 * time.Second)

	host := fmt.Sprintf("%s:3000", privateIP)

	// TCP health check, same pattern as LocalDockerProvider — works for any
	// challenge app regardless of its route structure.
	prewarmCtx, cancel := context.WithTimeout(ctx, time.Duration(p.cfg.PrewarmTimeoutSecs)*time.Second)
	defer cancel()
	if err := waitUntilHealthyTCP(prewarmCtx, host); err != nil {
		_ = p.StopContainer(context.Background(), taskArn)
		return nil, fmt.Errorf("container health check failed: %w", err)
	}

	slog.Info("ecs task ready", "task_arn", taskArn, "host", host, "challenge", challengeID)
	return &ContainerInfo{ID: taskArn, Host: host}, nil
}

// waitForRunningAndResolveIP polls DescribeTasks until the task is RUNNING,
// then extracts the private IP from its attached ENI. Fargate tasks don't
// expose an IP until the network interface is actually attached, which only
// happens once the task has progressed past PROVISIONING/PENDING.
func (p *awsECSProvider) waitForRunningAndResolveIP(ctx context.Context, taskArn string) (string, error) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timed out waiting for task to start: %w", ctx.Err())
		case <-ticker.C:
		}

		descResp, err := p.ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(p.cfg.ECSClusterName),
			Tasks:   []string{taskArn},
		})
		if err != nil {
			return "", fmt.Errorf("describe tasks: %w", err)
		}
		if len(descResp.Tasks) == 0 {
			continue
		}

		task := descResp.Tasks[0]
		status := aws.ToString(task.LastStatus)

		if status == "STOPPED" {
			reason := aws.ToString(task.StoppedReason)
			return "", fmt.Errorf("task stopped before becoming healthy: %s", reason)
		}

		if status != "RUNNING" {
			continue
		}

		// Task is running — find the private IP from its ENI attachment.
		ip := extractPrivateIP(task)
		if ip == "" {
			continue // ENI attached but IP not yet reported; keep polling
		}
		return ip, nil
	}
}

// waitForExecAgentReady polls DescribeTasks until the ExecuteCommandAgent
// managed agent reports RUNNING. Task-level RUNNING only means the container
// process has started — the SSM exec agent sidecar initializes separately
// and can lag behind that by several seconds. Calling ExecCommand before
// this agent is ready produces InvalidParameterException: "execute command
// was not enabled... or the execute command agent isn't running."
func (p *awsECSProvider) waitForExecAgentReady(ctx context.Context, taskArn string) error {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for exec agent: %w", ctx.Err())
		case <-ticker.C:
		}

		descResp, err := p.ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(p.cfg.ECSClusterName),
			Tasks:   []string{taskArn},
		})
		if err != nil {
			return fmt.Errorf("describe tasks: %w", err)
		}
		if len(descResp.Tasks) == 0 {
			continue
		}

		task := descResp.Tasks[0]
		if len(task.Containers) == 0 {
			continue
		}

		for _, agent := range task.Containers[0].ManagedAgents {
			if string(agent.Name) == "ExecuteCommandAgent" &&
				aws.ToString(agent.LastStatus) == "RUNNING" {
				return nil
			}
		}
		// Agent not ready yet — keep polling
	}
}

// extractPrivateIP digs the private IPv4 address out of a task's network
// interface attachment details. ECS reports this as a key/value pair inside
// the attachment's Details list rather than a dedicated field.
func extractPrivateIP(task ecstypes.Task) string {
	for _, attachment := range task.Attachments {
		if aws.ToString(attachment.Type) != "ElasticNetworkInterface" {
			continue
		}
		for _, detail := range attachment.Details {
			if aws.ToString(detail.Name) == "privateIPv4Address" {
				return aws.ToString(detail.Value)
			}
		}
	}
	return ""
}

// StopContainer implements Provider. Stops the ECS task; ECS handles
// deprovisioning and billing cutoff once the task reaches STOPPED.
func (p *awsECSProvider) StopContainer(ctx context.Context, containerID string) error {
	_, err := p.ecsClient.StopTask(ctx, &ecs.StopTaskInput{
		Cluster: aws.String(p.cfg.ECSClusterName),
		Task:    aws.String(containerID),
		Reason:  aws.String("session ended"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("ecs stop task: %w", err)
	}
	return nil
}
