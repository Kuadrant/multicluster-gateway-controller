package dns

import (
	"context"
	"fmt"
	"strings"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/aws"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type providerFactory struct {
	client client.Client
}

func NewProvider(c client.Client) *providerFactory {

	return &providerFactory{
		client: c,
	}
}

var unSupportedProviderErr = fmt.Errorf("unsupported provider")

func IsUnsupportedErr(err error) bool {
	return err == unSupportedProviderErr
}

func (pf *providerFactory) Factory(ctx context.Context, zone *v1alpha1.ManagedZone) (Provider, error) {
	prov, cred, err := pf.loadProvider(ctx, zone)
	if err != nil {
		return nil, fmt.Errorf("failed to load provider and provider credential for zone %s : %s", zone.Name, err)
	}
	switch strings.ToUpper(prov.Type) {
	case "AWS":
		return aws.NewProviderFromSecret(cred)
	default:
		return nil, unSupportedProviderErr
	}
}

// in AWS package
func ProviderFromSecret(creds *v1.Secret) (Provider, error) {
	return nil, nil
}

func (pf *providerFactory) loadProvider(ctx context.Context, zone *v1alpha1.ManagedZone) (*v1alpha1.DNSProvider, *v1.Secret, error) {
	provider := &v1alpha1.DNSProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      zone.Spec.ProviderRef.Name,
			Namespace: zone.Spec.ProviderRef.Namespace,
		},
	}
	if err := pf.client.Get(ctx, client.ObjectKeyFromObject(provider), provider); err != nil {
		return nil, nil, err
	}
	providerSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      provider.Spec.Credentials.Name,
			Namespace: provider.Spec.Credentials.Namespace,
		},
	}
	if err := pf.client.Get(ctx, client.ObjectKeyFromObject(providerSecret), providerSecret); err != nil {
		return nil, nil, err
	}

	return provider, providerSecret, nil
}
