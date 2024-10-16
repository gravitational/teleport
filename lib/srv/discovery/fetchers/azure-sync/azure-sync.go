package azure_sync

import (
	"context"
	accessgraphv1alpha "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
)

type Config struct {
	Regions             []string
	Integration         string
	DiscoveryConfigName string
}

type Resources struct {
	VirtualMachines []*accessgraphv1alpha.AzureVirtualMachine
}
type Features struct {
	VirtualMachines bool
}
type AzureFetcher interface {
	Poll(context.Context, Features) (*Resources, error)
	Status() (uint64, error)
	DiscoveryConfigName() string
	IsFromDiscoveryConfig() bool
	GetAccountID() string
}
type azureFetcher struct {
	Config
	lastError               error
	lastDiscoveredResources uint64
	lastResult              *Resources
}

func NewAzureFetcher(cfg Config) (AzureFetcher, error) {
	return &azureFetcher{
		Config: cfg,
	}, nil
}
func (a *azureFetcher) Poll(context.Context, Features) (*Resources, error) {
	return nil, nil
}
func (a *azureFetcher) Status() (uint64, error) {
	return 0, nil
}
func (a *azureFetcher) DiscoveryConfigName() string {
	return ""
}
func (a *azureFetcher) IsFromDiscoveryConfig() bool {
	return false
}
func (a *azureFetcher) GetAccountID() string {
	return ""
}
