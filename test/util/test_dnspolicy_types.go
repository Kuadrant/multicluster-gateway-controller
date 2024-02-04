//go:build unit || integration || e2e

package testutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	kuadrantdnsv1alpha1 "github.com/kuadrant/kuadrant-dns-operator/api/v1alpha1"
	kuadrantv1alpha1 "github.com/kuadrant/kuadrant-operator/api/v1alpha1"
)

// DNSPolicyBuilder wrapper for DNSPolicy builder helper
type DNSPolicyBuilder struct {
	*kuadrantv1alpha1.DNSPolicy
}

func NewDNSPolicyBuilder(name, ns string) *DNSPolicyBuilder {
	return &DNSPolicyBuilder{
		&kuadrantv1alpha1.DNSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: kuadrantv1alpha1.DNSPolicySpec{},
		},
	}
}

func (t *DNSPolicyBuilder) WithTargetRef(targetRef gatewayapiv1alpha2.PolicyTargetReference) *DNSPolicyBuilder {
	t.Spec.TargetRef = targetRef
	return t
}

func (t *DNSPolicyBuilder) WithHealthCheck(healthCheck kuadrantv1alpha1.HealthCheckSpec) *DNSPolicyBuilder {
	t.Spec.HealthCheck = &healthCheck
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancing(loadBalancing kuadrantv1alpha1.LoadBalancingSpec) *DNSPolicyBuilder {
	t.Spec.LoadBalancing = &loadBalancing
	return t
}

func (t *DNSPolicyBuilder) WithRoutingStrategy(strategy kuadrantv1alpha1.RoutingStrategy) *DNSPolicyBuilder {
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

//HealthCheck

func (t *DNSPolicyBuilder) WithHealthCheckFor(endpoint string, port *int, protocol kuadrantdnsv1alpha1.HealthProtocol, failureThreshold *int) *DNSPolicyBuilder {
	return t.WithHealthCheck(kuadrantv1alpha1.HealthCheckSpec{
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

func (t *DNSPolicyBuilder) WithLoadBalancingWeighted(lbWeighted kuadrantv1alpha1.LoadBalancingWeighted) *DNSPolicyBuilder {
	if t.Spec.LoadBalancing == nil {
		t.Spec.LoadBalancing = &kuadrantv1alpha1.LoadBalancingSpec{}
	}
	t.Spec.LoadBalancing.Weighted = &lbWeighted
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancingGeo(lbGeo kuadrantv1alpha1.LoadBalancingGeo) *DNSPolicyBuilder {
	if t.Spec.LoadBalancing == nil {
		t.Spec.LoadBalancing = &kuadrantv1alpha1.LoadBalancingSpec{}
	}
	t.Spec.LoadBalancing.Geo = &lbGeo
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancingWeightedFor(defaultWeight kuadrantv1alpha1.Weight, custom []*kuadrantv1alpha1.CustomWeight) *DNSPolicyBuilder {
	return t.WithLoadBalancingWeighted(kuadrantv1alpha1.LoadBalancingWeighted{
		DefaultWeight: defaultWeight,
		Custom:        custom,
	})
}

func (t *DNSPolicyBuilder) WithLoadBalancingGeoFor(defaultGeo string) *DNSPolicyBuilder {
	return t.WithLoadBalancingGeo(kuadrantv1alpha1.LoadBalancingGeo{
		DefaultGeo: defaultGeo,
	})
}

// ManagedZoneBuilder wrapper for ManagedZone builder helper
type ManagedZoneBuilder struct {
	*kuadrantdnsv1alpha1.ManagedZone
}

func NewManagedZoneBuilder(name, ns, domainName string) *ManagedZoneBuilder {
	return &ManagedZoneBuilder{
		&kuadrantdnsv1alpha1.ManagedZone{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: kuadrantdnsv1alpha1.ManagedZoneSpec{
				ID:          "1234",
				DomainName:  domainName,
				Description: domainName,
				SecretRef: kuadrantdnsv1alpha1.ProviderRef{
					Name: "secretname",
				},
			},
		},
	}
}
