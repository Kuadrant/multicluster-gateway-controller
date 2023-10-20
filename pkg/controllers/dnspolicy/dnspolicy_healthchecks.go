package dnspolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8serror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"
	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

func (r *DNSPolicyReconciler) reconcileHealthChecks(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
	log := crlog.FromContext(ctx)

	log.V(3).Info("reconciling health checks")
	for _, gw := range append(gwDiffObj.GatewaysWithValidPolicyRef, gwDiffObj.GatewaysMissingPolicyRef...) {
		log.V(3).Info("reconciling probes", "gateway", gw.Name)
		expectedProbes := r.expectedProbesForGateway(ctx, gw, dnsPolicy)
		if err := r.createOrUpdateProbes(ctx, expectedProbes); err != nil {
			return fmt.Errorf("error creating and updating expected proves for gateway %v: %w", gw.Gateway.Name, err)
		}
		if err := r.deleteUnexpectedGatewayProbes(ctx, expectedProbes, gw.Gateway, dnsPolicy); err != nil {
			return fmt.Errorf("error removing unexpected probes for gateway %v: %w", gw.Gateway.Name, err)
		}

	}

	for _, gw := range gwDiffObj.GatewaysWithInvalidPolicyRef {
		log.V(3).Info("deleting probes", "gateway", gw.Gateway.Name)
		if err := r.deleteGatewayProbes(ctx, gw.Gateway, dnsPolicy); err != nil {
			return fmt.Errorf("error deleting probes for gw %v: %w", gw.Gateway.Name, err)
		}
	}

	return nil
}

func (r *DNSPolicyReconciler) createOrUpdateProbes(ctx context.Context, expectedProbes []*v1alpha1.DNSHealthCheckProbe) error {
	//create or update all expected probes
	for _, hcProbe := range expectedProbes {
		p := &v1alpha1.DNSHealthCheckProbe{}
		if err := r.Client().Get(ctx, client.ObjectKeyFromObject(hcProbe), p); k8serror.IsNotFound(err) {
			if err := r.Client().Create(ctx, hcProbe); err != nil {
				return err
			}
		} else if client.IgnoreNotFound(err) == nil {
			p.Spec = hcProbe.Spec
			if err := r.Client().Update(ctx, p); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func (r *DNSPolicyReconciler) deleteGatewayProbes(ctx context.Context, gateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	return r.deleteProbesWithLabels(ctx, commonDNSRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy)), dnsPolicy.Namespace)
}

func (r *DNSPolicyReconciler) deleteProbes(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy) error {
	return r.deleteProbesWithLabels(ctx, policyDNSRecordLabels(client.ObjectKeyFromObject(dnsPolicy)), dnsPolicy.Namespace)
}

func (r *DNSPolicyReconciler) deleteProbesWithLabels(ctx context.Context, lbls map[string]string, namespace string) error {
	probes := &v1alpha1.DNSHealthCheckProbeList{}
	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(lbls), Namespace: namespace}
	if err := r.Client().List(ctx, probes, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}
	for _, p := range probes.Items {
		if err := r.Client().Delete(ctx, &p); err != nil {
			return err
		}
	}
	return nil
}

func (r *DNSPolicyReconciler) deleteUnexpectedGatewayProbes(ctx context.Context, expectedProbes []*v1alpha1.DNSHealthCheckProbe, gateway *gatewayv1beta1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	// remove any probes for this gateway and DNS Policy that are no longer expected
	existingProbes := &v1alpha1.DNSHealthCheckProbeList{}
	dnsLabels := commonDNSRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy))
	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(dnsLabels)}
	if err := r.Client().List(ctx, existingProbes, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	}
	for _, p := range existingProbes.Items {
		if !slice.Contains(expectedProbes, func(expectedProbe *v1alpha1.DNSHealthCheckProbe) bool {
			return expectedProbe.Name == p.Name && expectedProbe.Namespace == p.Namespace
		}) {
			if err := r.Client().Delete(ctx, &p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *DNSPolicyReconciler) expectedProbesForGateway(ctx context.Context, gw common.GatewayWrapper, dnsPolicy *v1alpha1.DNSPolicy) []*v1alpha1.DNSHealthCheckProbe {
	log := crlog.FromContext(ctx)
	var healthChecks []*v1alpha1.DNSHealthCheckProbe
	if dnsPolicy.Spec.HealthCheck == nil {
		log.V(3).Info("DNS Policy has no defined health check")
		return healthChecks
	}

	interval := metav1.Duration{Duration: 60 * time.Second}
	if dnsPolicy.Spec.HealthCheck.Interval != nil {
		interval = *dnsPolicy.Spec.HealthCheck.Interval
	}

	for _, address := range gw.Status.Addresses {
		matches := strings.Split(address.Value, "/")
		if len(matches) != 2 {
			log.V(3).Info(fmt.Sprintf("%s cannot be processed. expected <clsutername>/<hostname/ipaddress>", address.Value))
			return nil
		}

		for _, listener := range gw.Spec.Listeners {
			if strings.Contains(string(*listener.Hostname), "*") {
				continue
			}

			port := dnsPolicy.Spec.HealthCheck.Port
			if port == nil {
				listenerPort := int(listener.Port)
				port = &listenerPort
			}

			// handle protocol being nil
			var protocol string
			if dnsPolicy.Spec.HealthCheck.Protocol == nil {
				protocol = string(listener.Protocol)
			} else {
				protocol = string(*dnsPolicy.Spec.HealthCheck.Protocol)
			}
			log.V(1).Info("reconcileHealthChecks: adding health check for target", "target", address.Value)
			healthCheck := &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dnsHealthCheckProbeName(matches[1], gw.Name, string(listener.Name)),
					Namespace: gw.Namespace,
					Labels:    commonDNSRecordLabels(client.ObjectKeyFromObject(gw), client.ObjectKeyFromObject(dnsPolicy)),
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					Port:                     *port,
					Host:                     string(*listener.Hostname),
					Address:                  matches[1],
					Path:                     dnsPolicy.Spec.HealthCheck.Endpoint,
					Protocol:                 v1alpha1.HealthProtocol(protocol),
					Interval:                 interval,
					AdditionalHeadersRef:     dnsPolicy.Spec.HealthCheck.AdditionalHeadersRef,
					FailureThreshold:         dnsPolicy.Spec.HealthCheck.FailureThreshold,
					ExpectedResponses:        dnsPolicy.Spec.HealthCheck.ExpectedResponses,
					AllowInsecureCertificate: dnsPolicy.Spec.HealthCheck.AllowInsecureCertificates,
				},
			}
			healthChecks = append(healthChecks, withGatewayListener(gw, listener, healthCheck))
		}
	}

	return healthChecks
}

func dnsHealthCheckProbeName(address, gatewayName, listenerName string) string {
	return fmt.Sprintf("%s-%s", address, dnsRecordName(gatewayName, listenerName))
}
