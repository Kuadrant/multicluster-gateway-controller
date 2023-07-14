package dnsprovider

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/aws"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/google"
)

var errUnsupportedProvider = fmt.Errorf("provider type given is not supported")

type providerFactory struct {
	client.Client
}

func NewProvider(c client.Client) *providerFactory {

	return &providerFactory{
		Client: c,
	}
}

// depending on the provider type specified in the form of a custom secret type https://kubernetes.io/docs/concepts/configuration/secret/#secret-types in the dnsprovider secret it returns a dnsprovider.
func (p *providerFactory) DNSProviderFactory(ctx context.Context, managedZone *v1alpha1.ManagedZone) (dns.Provider, error) {
	providerSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedZone.Spec.SecretRef.Name,
			Namespace: managedZone.Spec.SecretRef.Namespace,
		}}

	if err := p.Client.Get(ctx, client.ObjectKeyFromObject(providerSecret), providerSecret); err != nil {
		return nil, err
	}

	switch providerSecret.Type {
	case "kuadrant.io/aws":
		dnsProvider, err := aws.NewProviderFromSecret(providerSecret)
		if err != nil {
			return nil, fmt.Errorf("unable to create dns provider from secret: %v", err)
		}
		log.Log.V(1).Info("Route53 provider created", "managed zone:", managedZone.Name)

		return dnsProvider, nil
	case "kuadrant.io/google":
		dnsProvider, err := google.NewProviderFromSecret(ctx, providerSecret)
		if err != nil {
			return nil, fmt.Errorf("unable to create dns provider from secret: %v", err)
		}
		log.Log.V(1).Info("Google provider created", "managed zone:", managedZone.Name)

		return dnsProvider, nil

	default:
		return nil, errUnsupportedProvider
	}

}
