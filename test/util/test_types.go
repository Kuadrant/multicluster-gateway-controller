//go:build unit || integration || e2e

package testutil

import (
	"strings"

	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayapiv1alpha2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
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

func NewTestGatewayClass(name, ns, controllerName string) *gatewayapiv1.GatewayClass {
	return &gatewayapiv1.GatewayClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: gatewayapiv1.GatewayClassSpec{
			ControllerName: gatewayapiv1.GatewayController(controllerName),
		},
	}
}

// GatewayBuilder wrapper for Gateway builder helper
type GatewayBuilder struct {
	*gatewayapiv1.Gateway
}

func NewGatewayBuilder(gwName, gwClassName, ns string) *GatewayBuilder {
	return &GatewayBuilder{
		&gatewayapiv1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      gwName,
				Namespace: ns,
			},
			Spec: gatewayapiv1.GatewaySpec{
				GatewayClassName: gatewayapiv1.ObjectName(gwClassName),
				Listeners:        []gatewayapiv1.Listener{},
			},
		},
	}
}

func (t *GatewayBuilder) WithListener(listener gatewayapiv1.Listener) *GatewayBuilder {
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
	typedHostname := gatewayapiv1.Hostname(hostname)
	t.WithListener(gatewayapiv1.Listener{
		Name:     gatewayapiv1.SectionName(name),
		Hostname: &typedHostname,
		Port:     gatewayapiv1.PortNumber(80),
		Protocol: gatewayapiv1.HTTPProtocolType,
	})
	return t
}

func (t *GatewayBuilder) WithHTTPSListener(hostname, tlsSecretName string) *GatewayBuilder {
	typedHostname := gatewayapiv1.Hostname(hostname)
	typedNamespace := gatewayapiv1.Namespace(t.GetNamespace())
	typedNamed := gatewayapiv1.SectionName(strings.Replace(hostname, "*", "wildcard", 1))
	t.WithListener(gatewayapiv1.Listener{
		Name:     typedNamed,
		Hostname: &typedHostname,
		Port:     gatewayapiv1.PortNumber(443),
		Protocol: gatewayapiv1.HTTPSProtocolType,
		TLS: &gatewayapiv1.GatewayTLSConfig{
			Mode: Pointer(gatewayapiv1.TLSModeTerminate),
			CertificateRefs: []gatewayapiv1.SecretObjectReference{
				{
					Name:      gatewayapiv1.ObjectName(tlsSecretName),
					Namespace: Pointer(typedNamespace),
				},
			},
		},
	})
	return t
}

func AddListener(name string, hostname gatewayapiv1alpha2.Hostname, secretName gatewayapiv1.ObjectName, gw *gatewayapiv1.Gateway) {
	listener := gatewayapiv1alpha2.Listener{
		Name:     gatewayapiv1.SectionName(name),
		Hostname: &hostname,
		Port:     443,
		Protocol: gatewayapiv1.HTTPSProtocolType,
		TLS: &gatewayapiv1.GatewayTLSConfig{
			CertificateRefs: []gatewayapiv1.SecretObjectReference{
				{
					Name: secretName,
				},
			},
		},
		AllowedRoutes: &gatewayapiv1.AllowedRoutes{
			Namespaces: &gatewayapiv1.RouteNamespaces{
				From: Pointer(gatewayapiv1.NamespacesFromAll),
			},
		},
	}
	gw.Spec.Listeners = append(gw.Spec.Listeners, listener)

}

//
//// TLSPolicyBuilder wrapper for TLSPolicy builder helper
//type TLSPolicyBuilder struct {
//	*v1alpha1.TLSPolicy
//}
//
//func NewTLSPolicyBuilder(policyName, ns string) *TLSPolicyBuilder {
//	return &TLSPolicyBuilder{
//		&v1alpha1.TLSPolicy{
//			ObjectMeta: metav1.ObjectMeta{
//				Name:      policyName,
//				Namespace: ns,
//			},
//			Spec: v1alpha1.TLSPolicySpec{},
//		},
//	}
//}
//
//func (t *TLSPolicyBuilder) Build() *v1alpha1.TLSPolicy {
//	return t.TLSPolicy
//}
//
//func (t *TLSPolicyBuilder) WithTargetGateway(gwName string) *TLSPolicyBuilder {
//	typedNamespace := gatewayapiv1.Namespace(t.GetNamespace())
//	t.Spec.TargetRef = gatewayapiv1alpha2.PolicyTargetReference{
//		Group:     "gateway.networking.k8s.io",
//		Kind:      "Gateway",
//		Name:      gatewayapiv1.ObjectName(gwName),
//		Namespace: &typedNamespace,
//	}
//	return t
//}
//
//func (t *TLSPolicyBuilder) WithIssuerRef(issuerRef certmanmetav1.ObjectReference) *TLSPolicyBuilder {
//	t.Spec.IssuerRef = issuerRef
//	return t
//}
//
//func (t *TLSPolicyBuilder) WithIssuer(name, kind, group string) *TLSPolicyBuilder {
//	t.WithIssuerRef(certmanmetav1.ObjectReference{
//		Name:  name,
//		Kind:  kind,
//		Group: group,
//	})
//	return t
//}

var _ client.Object = &TestResource{}

// TestResource dummy client.Object that can be used in place of a real k8s resource for testing
type TestResource struct {
	metav1.TypeMeta
	metav1.ObjectMeta
}

func (*TestResource) GetObjectKind() schema.ObjectKind { return nil }
func (*TestResource) DeepCopyObject() runtime.Object   { return nil }
