package traffic

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
)

func NewIngress(i *networkingv1.Ingress) *Ingress {
	return &Ingress{Ingress: i}
}

type Ingress struct {
	*networkingv1.Ingress
}

func (a *Ingress) GetKind() string {
	return "Ingress"
}

// GetDNSTargets will return the LB hosts and or IPs from the the Ingress object associated with the cluster they came from
func (a *Ingress) GetDNSTargets() []dns.Target {
	dnsTargets := []dns.Target{}
	for _, lb := range a.Status.LoadBalancer.Ingress {
		dnsTarget := dns.Target{}
		if lb.IP != "" {
			dnsTarget.TargetType = dns.TargetTypeIP
			dnsTarget.Value = lb.IP
		}
		if lb.Hostname != "" {
			dnsTarget.TargetType = dns.TargetTypeHost
			dnsTarget.Value = lb.Hostname

		}
		dnsTargets = append(dnsTargets, dnsTarget)
	}
	return dnsTargets
}

func (a *Ingress) GetHosts() []string {
	var hosts []string
	for _, rule := range a.Spec.Rules {
		if !slices.Contains(hosts, rule.Host) {
			hosts = append(hosts, rule.Host)
		}
	}

	return hosts
}

func (a *Ingress) AddTLS(host string, secret *corev1.Secret) {
	for i, tls := range a.Spec.TLS {
		if slice.ContainsString(tls.Hosts, host) {
			a.Spec.TLS[i] = networkingv1.IngressTLS{
				Hosts:      []string{host},
				SecretName: secret.Name,
			}
			return
		}
	}
	a.Spec.TLS = append(a.Spec.TLS, networkingv1.IngressTLS{
		Hosts:      []string{host},
		SecretName: secret.GetName(),
	})
}

func (a *Ingress) RemoveTLS(hosts []string) {
	for _, removeHost := range hosts {
		for i, tls := range a.Spec.TLS {
			tlsHosts := tls.Hosts
			for j, host := range tls.Hosts {
				if host == removeHost {
					tlsHosts = append(hosts[:j], hosts[j+1:]...)
				}
			}
			// if there are no hosts remaining remove the entry for TLS
			if len(tlsHosts) == 0 {
				a.Spec.TLS = append(a.Spec.TLS[:i], a.Spec.TLS[i+1:]...)
			}
		}
	}
}

func (a *Ingress) GetSpec() interface{} {
	return a.Spec
}

func (a *Ingress) GetNamespaceName() types.NamespacedName {
	return types.NamespacedName{
		Namespace: a.Namespace,
		Name:      a.Name,
	}
}

func (a *Ingress) GetCacheKey() string {
	key, _ := cache.MetaNamespaceKeyFunc(a)
	return key
}

func (a *Ingress) String() string {
	return fmt.Sprintf("kind: %v, namespace/name: %v", a.GetKind(), a.GetNamespaceName())
}
