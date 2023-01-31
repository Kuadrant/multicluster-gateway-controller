package traffic

import (
	"fmt"
	"strings"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/strings/slices"

	internalctrl "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/controller"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/slice"
	kuadrantv1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1"
)

const (
	AnnotationManagedHosts = "kuadrant.io/managed-hosts"
)

func NewIngress(i *networkingv1.Ingress) Interface {
	return &Ingress{Ingress: i}
}

type Ingress struct {
	*networkingv1.Ingress
}

func (a *Ingress) GetKind() string {
	return "Ingress"
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

func (a *Ingress) AddManagedHost(h string) error {
	// rules to add to the spec
	additionalRules := []networkingv1.IngressRule{}
	// rules we have covered already in the spec
	coveredRules := []*networkingv1.HTTPIngressRuleValue{}
	for _, existing := range a.Spec.Rules {
		if existing.Host == h {
			coveredRules = append(coveredRules, existing.HTTP)
		}
	}

	var isCovered = func(val *networkingv1.HTTPIngressRuleValue) bool {
		for _, ar := range coveredRules {
			if equality.Semantic.DeepEqual(ar, val) {
				return true
			}
		}
		return false
	}
	// we now know what rules we have already covered so now calculate any new ones
	for _, existing := range a.Spec.Rules {
		if existing.Host == h || isCovered(existing.HTTP) {
			continue
		}

		additional := existing.DeepCopy()
		additional.Host = h
		additionalRules = append(additionalRules, *additional)
		coveredRules = append(coveredRules, additional.HTTP)
	}
	a.Spec.Rules = append(a.Spec.Rules, additionalRules...)
	if a.Annotations == nil {
		a.Annotations = map[string]string{}
	}
	value := h
	if v, ok := a.Annotations[AnnotationManagedHosts]; ok {
		if v != "" {
			managedHosts := strings.Split(v, ",")
			for _, mh := range managedHosts {
				if mh == h {
					return nil
				}
			}
			v = fmt.Sprintf("%s,%s", v, h)
		}
		value = v
	}
	a.Annotations[AnnotationManagedHosts] = value
	return nil
}

func (a *Ingress) HasTLS() bool {
	return a.Spec.TLS != nil && len(a.Spec.TLS) != 0
}

func (a *Ingress) GetTLS() []TLSConfig {
	tls := []TLSConfig{}

	for _, section := range a.Spec.TLS {
		tls = append(tls, TLSConfig{
			Hosts:      section.Hosts,
			SecretName: section.SecretName,
		})
	}

	return tls
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

// GetDNSTargets will return the LB hosts and or IPs from the the Ingress object associated with the cluster they came from
func (a *Ingress) GetDNSTargets() ([]kuadrantv1.Target, error) {
	status := a.Status

	dnsTargets := []kuadrantv1.Target{}
	for _, lb := range status.LoadBalancer.Ingress {
		dnsTarget := kuadrantv1.Target{}
		//dnsTarget.Cluster = cluster.String()
		if lb.IP != "" {
			dnsTarget.TargetType = kuadrantv1.TargetTypeIP
			dnsTarget.Value = lb.IP
		}
		if lb.Hostname != "" {
			dnsTarget.TargetType = kuadrantv1.TargetTypeHost
			dnsTarget.Value = lb.Hostname

		}
		dnsTargets = append(dnsTargets, dnsTarget)
	}

	return dnsTargets, nil
}

func (a *Ingress) GetWebhookConfigurations(host string, caBundle []byte) ([]*admissionv1.ValidatingWebhookConfiguration, []*admissionv1.MutatingWebhookConfiguration) {
	var matchPolicy admissionv1.MatchPolicyType = admissionv1.Exact
	var scope admissionv1.ScopeType = admissionv1.AllScopes
	var sideEffects admissionv1.SideEffectClass = admissionv1.SideEffectClassNoneOnDryRun
	var timeoutSeconds int32 = 5
	var failurePolicy = admissionv1.Fail
	if internalctrl.IsRunningLocally() {
		failurePolicy = admissionv1.Ignore
	}

	url := fmt.Sprintf("https://%s/ingress", host)

	return []*admissionv1.ValidatingWebhookConfiguration{},
		[]*admissionv1.MutatingWebhookConfiguration{
			{
				ObjectMeta: v1.ObjectMeta{
					Name: "mctc",
				},

				Webhooks: []admissionv1.MutatingWebhook{
					{
						FailurePolicy: &failurePolicy,
						MatchPolicy:   &matchPolicy,
						Name:          "mctc.ingress.dev",
						ClientConfig: admissionv1.WebhookClientConfig{
							URL:      &url,
							CABundle: caBundle,
						},
						Rules: []admissionv1.RuleWithOperations{
							{
								Rule: admissionv1.Rule{
									APIGroups:   []string{"networking.k8s.io"},
									APIVersions: []string{"v1"},
									Resources:   []string{"ingresses", "ingresses/status"},
									Scope:       &scope,
								},
								Operations: []admissionv1.OperationType{
									admissionv1.Create,
									admissionv1.Update,
								},
							},
						},
						SideEffects:             &sideEffects,
						TimeoutSeconds:          &timeoutSeconds,
						AdmissionReviewVersions: []string{"v1"},
					},
				},
			},
		}
}

func (a *Ingress) ExposesOwnController() bool {
	if a.Annotations == nil {
		return false
	}

	component, ok := a.Annotations["mctc-component"]
	if !ok {
		return false
	}

	return component == "webhook"
}
