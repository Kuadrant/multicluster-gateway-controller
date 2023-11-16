//go:build unit || integration || e2e

package testutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

// DNSPolicyBuilder wrapper for DNSPolicy builder helper
type DNSPolicyBuilder struct {
	*v1alpha1.DNSPolicy
}

func NewDNSPolicyBuilder(name, ns string) *DNSPolicyBuilder {
	return &DNSPolicyBuilder{
		&v1alpha1.DNSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: v1alpha1.DNSPolicySpec{},
		},
	}
}

func (t *DNSPolicyBuilder) WithTargetRef(targetRef gatewayapiv1alpha2.PolicyTargetReference) *DNSPolicyBuilder {
	t.Spec.TargetRef = targetRef
	return t
}

func (t *DNSPolicyBuilder) WithHealthCheck(healthCheck v1alpha1.HealthCheckSpec) *DNSPolicyBuilder {
	t.Spec.HealthCheck = &healthCheck
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancing(loadBalancing v1alpha1.LoadBalancingSpec) *DNSPolicyBuilder {
	t.Spec.LoadBalancing = &loadBalancing
	return t
}

func (t *DNSPolicyBuilder) WithRoutingStrategy(strategy v1alpha1.RoutingStrategy) *DNSPolicyBuilder {
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

func (t *DNSPolicyBuilder) WithHealthCheckFor(endpoint string, port *int, protocol v1alpha1.HealthProtocol, failureThreshold *int) *DNSPolicyBuilder {
	return t.WithHealthCheck(v1alpha1.HealthCheckSpec{
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

func (t *DNSPolicyBuilder) WithLoadBalancingWeighted(lbWeighted v1alpha1.LoadBalancingWeighted) *DNSPolicyBuilder {
	if t.Spec.LoadBalancing == nil {
		t.Spec.LoadBalancing = &v1alpha1.LoadBalancingSpec{}
	}
	t.Spec.LoadBalancing.Weighted = &lbWeighted
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancingGeo(lbGeo v1alpha1.LoadBalancingGeo) *DNSPolicyBuilder {
	if t.Spec.LoadBalancing == nil {
		t.Spec.LoadBalancing = &v1alpha1.LoadBalancingSpec{}
	}
	t.Spec.LoadBalancing.Geo = &lbGeo
	return t
}

func (t *DNSPolicyBuilder) WithLoadBalancingWeightedFor(defaultWeight v1alpha1.Weight, custom []*v1alpha1.CustomWeight) *DNSPolicyBuilder {
	return t.WithLoadBalancingWeighted(v1alpha1.LoadBalancingWeighted{
		DefaultWeight: defaultWeight,
		Custom:        custom,
	})
}

func (t *DNSPolicyBuilder) WithLoadBalancingGeoFor(defaultGeo string) *DNSPolicyBuilder {
	return t.WithLoadBalancingGeo(v1alpha1.LoadBalancingGeo{
		DefaultGeo: defaultGeo,
	})
}

// ManagedZoneBuilder wrapper for ManagedZone builder helper
type ManagedZoneBuilder struct {
	*v1alpha1.ManagedZone
}

func NewManagedZoneBuilder(name, ns, domainName string) *ManagedZoneBuilder {
	return &ManagedZoneBuilder{
		&v1alpha1.ManagedZone{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: ns,
			},
			Spec: v1alpha1.ManagedZoneSpec{
				ID:          "1234",
				DomainName:  domainName,
				Description: domainName,
				SecretRef: &v1alpha1.SecretRef{
					Name:      "secretname",
					Namespace: ns,
				},
			},
		},
	}
}
