package dnsprovider

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/aws"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/google"
)

var (
	ErrUnsupportedProviderKind = fmt.Errorf("unsupported provider kind")
	ErrUnsupportedProviderType = fmt.Errorf("unsupported provider type")
)

type providerFactory struct {
	client.Client
}

func NewProvider(c client.Client) *providerFactory {

	return &providerFactory{
		Client: c,
	}
}

// depending on the provider type specified in the form of a custom secret type https://kubernetes.io/docs/concepts/configuration/secret/#secret-types in the dnsprovider secret it returns a dnsprovider.
func (p *providerFactory) DNSProviderFactory(ctx context.Context, pa v1alpha2.ProviderAccessor) (dns.Provider, error) {
	return p.provider(ctx, pa)
}

func (p *providerFactory) provider(ctx context.Context, pa v1alpha2.ProviderAccessor) (dns.Provider, error) {
	switch pa.GetProviderRef().Kind {
	case v1alpha2.ProviderKindSecret:
		return p.providerFromSecret(ctx, pa.GetProviderRef().Name, pa.GetNamespace())
	case v1alpha2.ProviderKindManagedZone:
		return p.providerFromManagedZone(ctx, pa.GetProviderRef().Name, pa.GetNamespace())
	case v1alpha2.ProviderKindNone:
		fallthrough
	default:
		return nil, fmt.Errorf("%w : %s", ErrUnsupportedProviderKind, pa.GetProviderRef().Kind)
	}
}

func (p *providerFactory) providerFromSecret(ctx context.Context, name, namespace string) (dns.Provider, error) {
	var providerSecret v1.Secret
	if err := p.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &providerSecret); err != nil {
		return nil, err
	}

	switch providerSecret.Type {
	case "kuadrant.io/aws":
		dnsProvider, err := aws.NewProviderFromSecret(&providerSecret)
		if err != nil {
			return nil, fmt.Errorf("unable to create AWS dns provider from secret: %v", err)
		}
		log.Log.V(1).Info("Route53 provider created from secret", "name", name, "namespace", namespace)

		return dnsProvider, nil
	case "kuadrant.io/gcp":
		dnsProvider, err := google.NewProviderFromSecret(ctx, &providerSecret)
		if err != nil {
			return nil, fmt.Errorf("unable to create GCP dns provider from secret: %v", err)
		}
		log.Log.V(1).Info("Google provider created from secret", "name", name, "namespace", namespace)

		return dnsProvider, nil

	default:
		return nil, fmt.Errorf("%w : %s", ErrUnsupportedProviderType, providerSecret.Type)
	}
}

func (p *providerFactory) providerFromManagedZone(ctx context.Context, name, namespace string) (dns.Provider, error) {
	var mz v1alpha2.ManagedZone
	if err := p.Client.Get(ctx, client.ObjectKey{Name: name, Namespace: namespace}, &mz); err != nil {
		return nil, err
	}
	//Avoid ending up in a loop, a managed zone should not reference another managed zone
	if mz.GetProviderRef().Kind == v1alpha2.ProviderKindManagedZone {
		return nil, fmt.Errorf("%w : managed zone cannot have a providerRef with kind %s", ErrUnsupportedProviderKind, v1alpha2.ProviderKindManagedZone)
	}
	return p.provider(ctx, &mz)
}
