package dnsprovider

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns/aws"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var DNSProvider dns.Provider

type provider struct {
	client client.Client
}

func NewProvider(c client.Client) *provider {

	return &provider{
		client: c,
	}
}

func (p *provider) dnsProviderSecret(ctx context.Context, managedZone *v1alpha1.ManagedZone) (*v1alpha1.DNSProviderConfig, error) {
	dnsprovider := &v1alpha1.DNSProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedZone.Spec.ProviderRef.Name,
			Namespace: managedZone.Spec.ProviderRef.Namespace,
		}}
	log.Log.Info("Reconciling DNS Provider:", "Name:", dnsprovider.Name)
	err := p.client.Get(ctx, client.ObjectKeyFromObject(dnsprovider), dnsprovider)
	if err != nil {
		return nil, err
	}

	providerSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dnsprovider.Spec.Credentials.Name,
			Namespace: dnsprovider.Spec.Credentials.Namespace,
		}}
	log.Log.Info("Reconciling DNS provider secret:", "secretName", providerSecret.Name)
	err = p.client.Get(ctx, client.ObjectKeyFromObject(providerSecret), providerSecret)
	if err != nil {
		return nil, err
	}
	aws := &v1alpha1.DNSProviderConfig{}

	if len(providerSecret.Data) == 0 {
		log.Log.Info("Secret is empty")
		return nil, nil
	}
	if aws.Route53 == nil {
		aws.Route53 = &v1alpha1.DNSProviderConfigRoute53{}
	}

	if managedZone.Spec.ProviderRef.ProviderType == "AWS" {

		if value, ok := providerSecret.Data["AWS_SECRET_ACCESS_KEY"]; ok {
			aws.Route53.SecretAccessKey = string(value)
		}

		if value, ok := providerSecret.Data["AWS_ACCESS_KEY_ID"]; ok {
			aws.Route53.AccessKeyID = string(value)
		}

		if value, ok := providerSecret.Data["REGION"]; ok {
			aws.Route53.Region = string(value)
		}
	}

	return aws, nil
}

func (p *provider) CreateDNSProvider(ctx context.Context, managedZone *v1alpha1.ManagedZone) (dns.Provider, error) {
	creds, err := p.dnsProviderSecret(ctx, managedZone)
	if err != nil {
		return nil, err
	}

	switch managedZone.Spec.ProviderRef.ProviderType {
	case "AWS":
		log.Log.Info("Creating DNS provider for provider type AWS")
		DNSProvider, _ = aws.NewDNSProvider(*creds)
		return DNSProvider, nil

	case "GCP":
		log.Log.Info("GCP")

	case "AZURE":
		log.Log.Info("AZURE")

	default:
		log.Log.Info("No DNS provider found")
		return nil, nil
	}

	return DNSProvider, nil
}
