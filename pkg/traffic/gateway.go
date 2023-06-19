package traffic

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
)

type GatewayInterface interface {
	Interface
	GetListenerByHost(host string) *gatewayv1beta1.Listener
}

func NewGateway(g *gatewayv1beta1.Gateway) GatewayInterface {
	return &Gateway{Gateway: g}
}

type Gateway struct {
	*gatewayv1beta1.Gateway
}

func (a *Gateway) GetKind() string {
	return "Gateway"
}

func (a *Gateway) GetHosts() []string {
	var hosts []string
	for _, listener := range a.Spec.Listeners {
		host := (*string)(listener.Hostname)
		if host == nil {
			continue
		}
		if !slices.Contains(hosts, *host) {
			hosts = append(hosts, *host)
		}
	}

	return hosts
}

func (a *Gateway) HasTLS() bool {
	hasTLS := false
	for _, listener := range a.Spec.Listeners {
		if listener.TLS != nil {
			hasTLS = true
			break
		}
	}
	return hasTLS
}

func (a *Gateway) AddTLS(host string, secret *corev1.Secret) {
	listeners := []gatewayv1beta1.Listener{}

	gatewayNS := gatewayv1beta1.Namespace(a.Namespace)
	secretKind := gatewayv1beta1.Kind(secret.Kind)
	secretGroup := gatewayv1beta1.Group("")
	modeTerminate := gatewayv1beta1.TLSModeTerminate
	for _, listener := range a.Spec.Listeners {
		if *(*string)(listener.Hostname) == host {
			listener.TLS = &gatewayv1beta1.GatewayTLSConfig{
				Mode: &modeTerminate, // Ensure terminate mode as we're managing the cert
				CertificateRefs: []gatewayv1beta1.SecretObjectReference{
					{
						Group:     &secretGroup,
						Kind:      &secretKind,
						Name:      gatewayv1beta1.ObjectName(secret.Name),
						Namespace: &gatewayNS,
					},
				},
			}
		}
		listeners = append(listeners, listener)
	}

	a.Spec.Listeners = listeners
}

func (a *Gateway) RemoveTLS(hosts []string) {
	for _, listener := range a.Spec.Listeners {
		if slice.ContainsString(hosts, fmt.Sprint(listener.Hostname)) {
			listener.TLS = nil
		}
	}
}

func (a *Gateway) GetSpec() interface{} {
	return a.Spec
}

func (a *Gateway) GetNamespaceName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: a.Namespace,
		Name:      a.Name,
	}
}

func (a *Gateway) GetCacheKey() string {
	key, _ := cache.MetaNamespaceKeyFunc(a)
	return key
}

func (a *Gateway) String() string {
	return fmt.Sprintf("kind: %v, namespace/name: %v", a.GetKind(), a.GetNamespaceName())
}

func (a *Gateway) GetListenerByHost(host string) *gatewayv1beta1.Listener {
	for _, listener := range a.Spec.Listeners {
		if *(*string)(listener.Hostname) == host {
			return &listener
		}
	}
	return nil
}

func (a *Gateway) ExposesOwnController() bool {
	return false
}
