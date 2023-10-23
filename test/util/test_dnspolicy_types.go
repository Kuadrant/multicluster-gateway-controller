//go:build unit || integration || e2e

package testutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
)

// DNSPolicyBuilder wrapper for DNSPolicy builder helper
type DNSPolicyBuilder struct {
	*v1alpha2.DNSPolicy
}

func NewDNSPolicyBuilder(name, ns string) *DNSPolicyBuilder {
	return &DNSPolicyBuilder{
		&v1alpha2.DNSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: v1alpha2.DNSPolicySpec{},
		},
	}
}

func (t *DNSPolicyBuilder) WithTargetRef(targetRef gatewayapiv1alpha2.PolicyTargetReference) *DNSPolicyBuilder {
	t.Spec.TargetRef = targetRef
	return t
}

func (t *DNSPolicyBuilder) WithProviderRef(providerRef v1alpha2.ProviderRef) *DNSPolicyBuilder {
	t.Spec.ProviderRef = providerRef
	return t
}

func (t *DNSPolicyBuilder) WithHealthCheck(healthCheck v1alpha2.HealthCheckSpec) *DNSPolicyBuilder {
	t.Spec.HealthCheck = &healthCheck
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancing(loadBalancing v1alpha2.LoadBalancingSpec) *DNSPolicyBuilder {
	t.Spec.LoadBalancing = &loadBalancing
	return t
}

func (t *DNSPolicyBuilder) WithRoutingStrategy(strategy v1alpha2.RoutingStrategy) *DNSPolicyBuilder {
	t.Spec.RoutingStrategy = strategy
	return t
}

//TargetRef

func (t *DNSPolicyBuilder) WithTargetGateway(gwName string) *DNSPolicyBuilder {
	typedNamespace := gatewayapiv1.Namespace(t.GetNamespace())
	return t.WithTargetRef(gatewayapiv1alpha2.PolicyTargetReference{
		Group:     "gateway.networking.k8s.io",
		Kind:      "Gateway",
		Name:      gatewayapiv1.ObjectName(gwName),
		Namespace: &typedNamespace,
	})
}

//ProviderRef

func (t *DNSPolicyBuilder) WithProviderManagedZone(mzName string) *DNSPolicyBuilder {
	return t.WithProviderRef(v1alpha2.ProviderRef{
		Name: mzName,
		Kind: v1alpha2.ProviderKindManagedZone,
	})
}

func (t *DNSPolicyBuilder) WithProviderSecret(secretName string) *DNSPolicyBuilder {
	return t.WithProviderRef(v1alpha2.ProviderRef{
		Name: secretName,
		Kind: v1alpha2.ProviderKindSecret,
	})
}

func (t *DNSPolicyBuilder) WithProviderNone(name string) *DNSPolicyBuilder {
	return t.WithProviderRef(v1alpha2.ProviderRef{
		Name: name,
		Kind: v1alpha2.ProviderKindNone,
	})
}

//HealthCheck

func (t *DNSPolicyBuilder) WithHealthCheckFor(endpoint string, port *int, protocol v1alpha1.HealthProtocol, failureThreshold *int) *DNSPolicyBuilder {
	return t.WithHealthCheck(v1alpha2.HealthCheckSpec{
		Endpoint:                  endpoint,
		Port:                      port,
		Protocol:                  &protocol,
		FailureThreshold:          failureThreshold,
		AdditionalHeadersRef:      nil,
		ExpectedResponses:         nil,
		AllowInsecureCertificates: false,
		Interval:                  nil,
	})
}

//LoadBalancing

func (t *DNSPolicyBuilder) WithLoadBalancingWeighted(lbWeighted v1alpha2.LoadBalancingWeighted) *DNSPolicyBuilder {
	if t.Spec.LoadBalancing == nil {
		t.Spec.LoadBalancing = &v1alpha2.LoadBalancingSpec{}
	}
	t.Spec.LoadBalancing.Weighted = &lbWeighted
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancingGeo(lbGeo v1alpha2.LoadBalancingGeo) *DNSPolicyBuilder {
	if t.Spec.LoadBalancing == nil {
		t.Spec.LoadBalancing = &v1alpha2.LoadBalancingSpec{}
	}
	t.Spec.LoadBalancing.Geo = &lbGeo
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancingWeightedFor(defaultWeight v1alpha2.Weight, custom []*v1alpha2.CustomWeight) *DNSPolicyBuilder {
	return t.WithLoadBalancingWeighted(v1alpha2.LoadBalancingWeighted{
		DefaultWeight: defaultWeight,
		Custom:        custom,
	})
}

func (t *DNSPolicyBuilder) WithLoadBalancingGeoFor(defaultGeo string) *DNSPolicyBuilder {
	return t.WithLoadBalancingGeo(v1alpha2.LoadBalancingGeo{
		DefaultGeo: defaultGeo,
	})
}

// ManagedZoneBuilder wrapper for ManagedZone builder helper
type ManagedZoneBuilder struct {
	*v1alpha2.ManagedZone
}

func NewManagedZoneBuilder(name, ns string) *ManagedZoneBuilder {
	return &ManagedZoneBuilder{
		&v1alpha2.ManagedZone{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: v1alpha2.ManagedZoneSpec{},
		},
	}
}

func (t *ManagedZoneBuilder) WithID(id string) *ManagedZoneBuilder {
	t.Spec.ID = &id
	return t
}

func (t *ManagedZoneBuilder) WithDomainName(domainName string) *ManagedZoneBuilder {
	t.Spec.DomainName = domainName
	return t
}

func (t *ManagedZoneBuilder) WithDescription(description string) *ManagedZoneBuilder {
	t.Spec.Description = &description
	return t
}

func (t *ManagedZoneBuilder) WithProviderRef(providerRef v1alpha2.ProviderRef) *ManagedZoneBuilder {
	t.Spec.ProviderRef = providerRef
	return t
}

func (t *ManagedZoneBuilder) WithProviderManagedZone(mzName string) *ManagedZoneBuilder {
	return t.WithProviderRef(v1alpha2.ProviderRef{
		Name: mzName,
		Kind: v1alpha2.ProviderKindManagedZone,
	})
}

func (t *ManagedZoneBuilder) WithProviderSecret(secretName string) *ManagedZoneBuilder {
	return t.WithProviderRef(v1alpha2.ProviderRef{
		Name: secretName,
		Kind: v1alpha2.ProviderKindSecret,
	})
}

func (t *ManagedZoneBuilder) WithProviderNone(secretName string) *ManagedZoneBuilder {
	return t.WithProviderRef(v1alpha2.ProviderRef{
		Name: secretName,
		Kind: v1alpha2.ProviderKindNone,
	})
}
