//go:build unit || integration

package testutil

import (
	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

type TestGateway struct {
	*gatewayv1beta1.Gateway
}

func NewTestGateway(gwName, gwClassName, ns string) *TestGateway {
	return &TestGateway{
		&gatewayv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gwName,
				Namespace: ns,
			},
			Spec: gatewayv1beta1.GatewaySpec{
				GatewayClassName: gatewayv1beta1.ObjectName(gwClassName),
				Listeners:        []gatewayv1beta1.Listener{},
			},
		},
	}
}

func NewTestIssuer(name, ns string) *certmanv1.Issuer {
	return &certmanv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
	}
}

func NewTestClusterIssuer(name string) *certmanv1.ClusterIssuer {
	return &certmanv1.ClusterIssuer{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func (t *TestGateway) WithListener(listener gatewayv1beta1.Listener) *TestGateway {
	t.Spec.Listeners = append(t.Spec.Listeners, listener)
	return t
}

func (t *TestGateway) WithHTTPListener(hostname string) *TestGateway {
	typedHostname := gatewayv1beta1.Hostname(hostname)
	t.WithListener(gatewayv1beta1.Listener{
		Name:     gatewayv1beta1.SectionName(hostname),
		Hostname: &typedHostname,
		Port:     gatewayv1beta1.PortNumber(80),
		Protocol: gatewayv1beta1.HTTPProtocolType,
	})
	return t
}

func (t *TestGateway) WithHTTPSListener(hostname, tlsSecretName string) *TestGateway {
	typedHostname := gatewayv1beta1.Hostname(hostname)
	typedNamespace := gatewayv1beta1.Namespace(t.GetNamespace())
	t.WithListener(gatewayv1beta1.Listener{
		Name:     gatewayv1beta1.SectionName(hostname),
		Hostname: &typedHostname,
		Port:     gatewayv1beta1.PortNumber(443),
		Protocol: gatewayv1beta1.HTTPSProtocolType,
		TLS: &gatewayv1beta1.GatewayTLSConfig{
			Mode: Pointer(gatewayv1beta1.TLSModeTerminate),
			CertificateRefs: []gatewayv1beta1.SecretObjectReference{
				{
					Name:      gatewayv1beta1.ObjectName(tlsSecretName),
					Namespace: Pointer(typedNamespace),
				},
			},
		},
	})
	return t
}

type TestTLSPolicy struct {
	*v1alpha1.TLSPolicy
}

func NewTestTLSPolicy(policyName, ns string) *TestTLSPolicy {
	return &TestTLSPolicy{
		&v1alpha1.TLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: ns,
			},
			Spec: v1alpha1.TLSPolicySpec{},
		},
	}
}

func (t *TestTLSPolicy) WithTargetGateway(gwName string) *TestTLSPolicy {
	typedNamespace := gatewayv1beta1.Namespace(t.GetNamespace())
	t.Spec.TargetRef = gatewayapiv1alpha2.PolicyTargetReference{
		Group:     "gateway.networking.k8s.io",
		Kind:      "Gateway",
		Name:      gatewayv1beta1.ObjectName(gwName),
		Namespace: &typedNamespace,
	}
	return t
}

func (t *TestTLSPolicy) WithIssuerRef(issuerRef cmmeta.ObjectReference) *TestTLSPolicy {
	t.Spec.IssuerRef = issuerRef
	return t
}

func (t *TestTLSPolicy) WithIssuer(name, kind, group string) *TestTLSPolicy {
	t.WithIssuerRef(cmmeta.ObjectReference{
		Name:  name,
		Kind:  kind,
		Group: group,
	})
	return t
}

var _ client.Object = &TestResource{}

// TestResource dummy client.Object that can be used in place of a real k8s resource for testing
type TestResource struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (*TestResource) GetObjectKind() schema.ObjectKind { return nil }
func (*TestResource) DeepCopyObject() runtime.Object   { return nil }
