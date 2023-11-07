//go:build unit || integration || e2e

package testutil

import (
	"strings"

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

func NewTestGatewayClass(name, ns, controllerName string) *gatewayv1beta1.GatewayClass {
	return &gatewayv1beta1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: gatewayv1beta1.GatewayClassSpec{
			ControllerName: gatewayv1beta1.GatewayController(controllerName),
		},
	}
}

// GatewayBuilder wrapper for Gateway builder helper
type GatewayBuilder struct {
	*gatewayv1beta1.Gateway
}

func NewGatewayBuilder(gwName, gwClassName, ns string) *GatewayBuilder {
	return &GatewayBuilder{
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

func (t *GatewayBuilder) WithListener(listener gatewayv1beta1.Listener) *GatewayBuilder {
	t.Spec.Listeners = append(t.Spec.Listeners, listener)
	return t
}

func (t *GatewayBuilder) WithLabels(labels map[string]string) *GatewayBuilder {
	if t.Labels == nil {
		t.Labels = map[string]string{}
	}
	for key, value := range labels {
		t.Labels[key] = value
	}
	return t
}

func (t *GatewayBuilder) WithHTTPListener(name, hostname string) *GatewayBuilder {
	typedHostname := gatewayv1beta1.Hostname(hostname)
	t.WithListener(gatewayv1beta1.Listener{
		Name:     gatewayv1beta1.SectionName(name),
		Hostname: &typedHostname,
		Port:     gatewayv1beta1.PortNumber(80),
		Protocol: gatewayv1beta1.HTTPProtocolType,
	})
	return t
}

func (t *GatewayBuilder) WithHTTPSListener(hostname, tlsSecretName string) *GatewayBuilder {
	typedHostname := gatewayv1beta1.Hostname(hostname)
	typedNamespace := gatewayv1beta1.Namespace(t.GetNamespace())
	typedNamed := gatewayv1beta1.SectionName(strings.Replace(hostname, "*", "wildcard", 1))
	t.WithListener(gatewayv1beta1.Listener{
		Name:     typedNamed,
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

func AddListener(name string, hostname gatewayapiv1alpha2.Hostname, secretName gatewayv1beta1.ObjectName, gw *gatewayv1beta1.Gateway) {
	listener := gatewayapiv1alpha2.Listener{
		Name:     gatewayv1beta1.SectionName(name),
		Hostname: &hostname,
		Port:     443,
		Protocol: gatewayv1beta1.HTTPSProtocolType,
		TLS: &gatewayv1beta1.GatewayTLSConfig{
			CertificateRefs: []gatewayv1beta1.SecretObjectReference{
				{
					Name: secretName,
				},
			},
		},
		AllowedRoutes: &gatewayv1beta1.AllowedRoutes{
			Namespaces: &gatewayv1beta1.RouteNamespaces{
				From: Pointer(gatewayv1beta1.NamespacesFromAll),
			},
		},
	}
	gw.Spec.Listeners = append(gw.Spec.Listeners, listener)

}

// TLSPolicyBuilder wrapper for TLSPolicy builder helper
type TLSPolicyBuilder struct {
	*v1alpha1.TLSPolicy
}

func NewTLSPolicyBuilder(policyName, ns string) *TLSPolicyBuilder {
	return &TLSPolicyBuilder{
		&v1alpha1.TLSPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      policyName,
				Namespace: ns,
			},
			Spec: v1alpha1.TLSPolicySpec{},
		},
	}
}

func (t *TLSPolicyBuilder) Build() *v1alpha1.TLSPolicy {
	return t.TLSPolicy
}

func (t *TLSPolicyBuilder) WithTargetGateway(gwName string) *TLSPolicyBuilder {
	typedNamespace := gatewayv1beta1.Namespace(t.GetNamespace())
	t.Spec.TargetRef = gatewayapiv1alpha2.PolicyTargetReference{
		Group:     "gateway.networking.k8s.io",
		Kind:      "Gateway",
		Name:      gatewayv1beta1.ObjectName(gwName),
		Namespace: &typedNamespace,
	}
	return t
}

func (t *TLSPolicyBuilder) WithIssuerRef(issuerRef cmmeta.ObjectReference) *TLSPolicyBuilder {
	t.Spec.IssuerRef = issuerRef
	return t
}

func (t *TLSPolicyBuilder) WithIssuer(name, kind, group string) *TLSPolicyBuilder {
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
