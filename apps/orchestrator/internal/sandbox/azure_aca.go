package sandbox

// import (
// 	"context"
// 	"fmt"
// )

// // azureContainerAppsProvider implements Provider using the Azure Container Apps REST API.
// // This provider is used in production (SANDBOX_PROVIDER=azure).
// //
// // Implementation status: STUB — not yet implemented.
// // Build LocalDockerProvider first, validate end-to-end, then implement this.
// //
// // Architecture notes for implementer:
// //   - Use the Azure SDK for Go (github.com/Azure/azure-sdk-for-go)
// //   - Each session creates a new Container App Job (not a persistent App)
// //   - Jobs are ephemeral: they start, run, and are cleaned up automatically
// //   - Terminal WebSocket: use ACA's exec endpoint (az containerapp exec equivalent)
// //   - Auth: use DefaultAzureCredential (works with managed identity in prod, env vars in dev)
// //   - Container image registry: Azure Container Registry (ACR)
// //   - Image naming convention: <acrName>.azurecr.io/stacktrace-<challengeID>:latest
// type azureContainerAppsProvider struct {
// 	cfg ProviderConfig
// }

// func newAzureContainerAppsProvider(cfg ProviderConfig) (*azureContainerAppsProvider, error) {
// 	// TODO: initialise Azure SDK clients
// 	// - azure.NewDefaultAzureCredential()
// 	// - armappcontainers.NewContainerAppsClient(subscriptionID, credential, nil)
// 	return &azureContainerAppsProvider{cfg: cfg}, nil
// }

// func (p *azureContainerAppsProvider) StartContainer(ctx context.Context, challengeID string) (*ContainerInfo, error) {
// 	return nil, fmt.Errorf("AzureContainerAppsProvider.StartContainer: not yet implemented")
// }

// func (p *azureContainerAppsProvider) StopContainer(ctx context.Context, containerID string) error {
// 	return fmt.Errorf("AzureContainerAppsProvider.StopContainer: not yet implemented")
// }

// func (p *azureContainerAppsProvider) WriteFile(ctx context.Context, containerID, relativePath, content string) error {
// 	return fmt.Errorf("AzureContainerAppsProvider.WriteFile: not yet implemented")
// }

// func (p *azureContainerAppsProvider) ReadFile(ctx context.Context, containerID, relativePath string) (string, error) {
// 	return "", fmt.Errorf("AzureContainerAppsProvider.ReadFile: not yet implemented")
// }

// func (p *azureContainerAppsProvider) ExecShell(ctx context.Context, containerID string) (ShellSession, error) {
// 	return nil, fmt.Errorf("AzureContainerAppsProvider.ExecShell: not yet implemented")
// }

// // Ensure compile-time interface satisfaction
// var _ Provider = (*azureContainerAppsProvider)(nil)
