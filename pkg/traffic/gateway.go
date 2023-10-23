package traffic

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
)

type GatewayInterface interface {
	Interface
	GetListenerByHost(host string) *gatewayapiv1.Listener
}

func NewGateway(g *gatewayapiv1.Gateway) GatewayInterface {
	return &Gateway{Gateway: g}
}

type Gateway struct {
	*gatewayapiv1.Gateway
}

func (g *Gateway) GetKind() string {
	return "Gateway"
}

func (g *Gateway) GetHosts() []string {
	var hosts []string
	for _, listener := range g.Spec.Listeners {
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

func (g *Gateway) HasTLS() bool {
	hasTLS := false
	for _, listener := range g.Spec.Listeners {
		if listener.TLS != nil {
			hasTLS = true
			break
		}
	}
	return hasTLS
}

func (g *Gateway) AddTLS(host string, secret *corev1.Secret) {
	listeners := []gatewayapiv1.Listener{}

	gatewayNS := gatewayapiv1.Namespace(g.Namespace)
	secretKind := gatewayapiv1.Kind(secret.Kind)
	secretGroup := gatewayapiv1.Group("")
	modeTerminate := gatewayapiv1.TLSModeTerminate
	for _, listener := range g.Spec.Listeners {
		if *(*string)(listener.Hostname) == host {
			listener.TLS = &gatewayapiv1.GatewayTLSConfig{
				Mode: &modeTerminate, // Ensure terminate mode as we're managing the cert
				CertificateRefs: []gatewayapiv1.SecretObjectReference{
					{
						Group:     &secretGroup,
						Kind:      &secretKind,
						Name:      gatewayapiv1.ObjectName(secret.Name),
						Namespace: &gatewayNS,
					},
				},
			}
		}
		listeners = append(listeners, listener)
	}

	g.Spec.Listeners = listeners
}

func (g *Gateway) RemoveTLS(hosts []string) {
	for _, listener := range g.Spec.Listeners {
		if slice.ContainsString(hosts, fmt.Sprint(listener.Hostname)) {
			listener.TLS = nil
		}
	}
}

func (g *Gateway) GetSpec() interface{} {
	return g.Spec
}

func (g *Gateway) GetNamespaceName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: g.Namespace,
		Name:      g.Name,
	}
}

func (g *Gateway) GetCacheKey() string {
	key, _ := cache.MetaNamespaceKeyFunc(g)
	return key
}

func (g *Gateway) String() string {
	return fmt.Sprintf("kind: %v, namespace/name: %v", g.GetKind(), g.GetNamespaceName())
}

func (g *Gateway) GetListenerByHost(host string) *gatewayapiv1.Listener {
	for _, listener := range g.Spec.Listeners {
		if *(*string)(listener.Hostname) == host {
			return &listener
		}
	}
	return nil
}

func (g *Gateway) ExposesOwnController() bool {
	return false
}
