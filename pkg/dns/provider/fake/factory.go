package fake

import (
	"context"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/provider"
)

type Factory struct {
	ProviderForFunc func(ctx context.Context, pa v1alpha1.ProviderAccessor) (provider.Provider, error)
}

var _ provider.Factory = &Factory{}

func (f *Factory) ProviderFor(ctx context.Context, pa v1alpha1.ProviderAccessor) (provider.Provider, error) {
	return f.ProviderForFunc(ctx, pa)
}
