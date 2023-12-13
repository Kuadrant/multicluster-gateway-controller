package dnspolicy

import (
	"context"
	"fmt"
	"strings"
	"time"

	k8serror "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/kuadrant-operator/pkg/common"
	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/utils"
)

func (r *DNSPolicyReconciler) reconcileHealthCheckProbes(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
	log := crlog.FromContext(ctx)

	log.V(3).Info("reconciling health checks")
	for _, gw := range gwDiffObj.GatewaysWithInvalidPolicyRef {
		log.V(1).Info("reconcileHealthCheckProbes: gateway with invalid policy ref", "key", gw.Key())
		if err := r.deleteGatewayHealthCheckProbes(ctx, gw.Gateway, dnsPolicy); err != nil {
			return fmt.Errorf("error deleting probes for gw %v: %w", gw.Gateway.Name, err)
		}
	}

	gwsWithIgnoredHosts := []string{}
	// Reconcile DNSHealthCheckProbes for each gateway directly referred by the policy (existing and new)
	for _, gw := range append(gwDiffObj.GatewaysWithValidPolicyRef, gwDiffObj.GatewaysMissingPolicyRef...) {
		log.V(3).Info("reconciling probes", "gateway", gw.Name)
		expectedProbes, ignoredHosts := r.expectedHealthCheckProbesForGateway(ctx, gw, dnsPolicy)
		if err := r.createOrUpdateHealthCheckProbes(ctx, expectedProbes); err != nil {
			return fmt.Errorf("error creating or updating expected probes for gateway %v: %w", gw.Gateway.Name, err)
		}
		if err := r.deleteUnexpectedGatewayHealthCheckProbes(ctx, expectedProbes, gw.Gateway, dnsPolicy); err != nil {
			return fmt.Errorf("error removing unexpected probes for gateway %v: %w", gw.Gateway.Name, err)
		}
		if ignoredHosts != nil {
			gwsWithIgnoredHosts = append(gwsWithIgnoredHosts, gw.Namespace+"/"+gw.Name)
		}
	}

	addHealthCheckCondition(dnsPolicy, gwsWithIgnoredHosts)

	return nil
}

func addHealthCheckCondition(dnsPolicy *v1alpha1.DNSPolicy, gwsWithIgnoredHosts []string) {
	if dnsPolicy.Status.HealthCheck == nil {
		dnsPolicy.Status.HealthCheck = &v1alpha1.HealthCheckStatus{}
	}

	noIgnoredHostsCondition := metav1.Condition{
		Type:               DNSPolicyHealthChecksNoIgnoredHosts,
		Status:             "True",
		ObservedGeneration: dnsPolicy.Generation,
		LastTransitionTime: metav1.Now(),
		Reason:             DNSPolicyHealthChecksIgnoredReason,
		Message:            "all gateways routes are creating health checks",
	}

	if len(gwsWithIgnoredHosts) > 0 {
		noIgnoredHostsCondition.Status = "False"
		noIgnoredHostsCondition.Reason = DNSPolicyHealthChecksIgnoredReason
		noIgnoredHostsCondition.Message = "gateways with ignored wildcard hosts: " + strings.Join(gwsWithIgnoredHosts, ", ")
	}

	meta.SetStatusCondition(&dnsPolicy.Status.HealthCheck.Conditions, noIgnoredHostsCondition)

}

func (r *DNSPolicyReconciler) createOrUpdateHealthCheckProbes(ctx context.Context, expectedProbes []*v1alpha1.DNSHealthCheckProbe) error {
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

func (r *DNSPolicyReconciler) deleteGatewayHealthCheckProbes(ctx context.Context, gateway *gatewayapiv1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
	return r.deleteHealthCheckProbesWithLabels(ctx, commonDNSRecordLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(dnsPolicy)), dnsPolicy.Namespace)
}

func (r *DNSPolicyReconciler) deleteHealthCheckProbes(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy) error {
	return r.deleteHealthCheckProbesWithLabels(ctx, policyDNSRecordLabels(client.ObjectKeyFromObject(dnsPolicy)), dnsPolicy.Namespace)
}

func (r *DNSPolicyReconciler) deleteHealthCheckProbesWithLabels(ctx context.Context, lbls map[string]string, namespace string) error {
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

func (r *DNSPolicyReconciler) deleteUnexpectedGatewayHealthCheckProbes(ctx context.Context, expectedProbes []*v1alpha1.DNSHealthCheckProbe, gateway *gatewayapiv1.Gateway, dnsPolicy *v1alpha1.DNSPolicy) error {
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

func (r *DNSPolicyReconciler) expectedHealthCheckProbesForGateway(ctx context.Context, gw common.GatewayWrapper, dnsPolicy *v1alpha1.DNSPolicy) ([]*v1alpha1.DNSHealthCheckProbe, []string) {
	log := crlog.FromContext(ctx)
	var healthChecks []*v1alpha1.DNSHealthCheckProbe
	var ignoredHosts []string
	if dnsPolicy.Spec.HealthCheck == nil {
		log.V(3).Info("DNS Policy has no defined health check")
		return healthChecks, ignoredHosts
	}

	interval := metav1.Duration{Duration: 60 * time.Second}
	if dnsPolicy.Spec.HealthCheck.Interval != nil {
		interval = *dnsPolicy.Spec.HealthCheck.Interval
	}

	gatewayWrapper := utils.NewGatewayWrapper(gw.Gateway)
	if err := gatewayWrapper.Validate(); err != nil {
		return nil, ignoredHosts
	}

	clusterGatewayAddresses := gatewayWrapper.GetClusterGatewayAddresses()

	for _, listener := range gw.Spec.Listeners {

		//skip wildcard listeners
		if strings.Contains(string(*listener.Hostname), "*") {
			ignoredHosts = append(ignoredHosts, string(*listener.Hostname))
			continue
		}

		port := dnsPolicy.Spec.HealthCheck.Port
		if port == nil {
			listenerPort := int(listener.Port)
			port = &listenerPort
		}

		var protocol string
		// handle protocol being nil
		if dnsPolicy.Spec.HealthCheck.Protocol == nil {
			protocol = string(listener.Protocol)
		} else {
			protocol = string(*dnsPolicy.Spec.HealthCheck.Protocol)
		}

		for _, addresses := range clusterGatewayAddresses {
			for _, address := range addresses {
				log.V(1).Info("reconcileHealthCheckProbes: adding health check for target", "target", address.Value)
				healthCheck := &v1alpha1.DNSHealthCheckProbe{
					ObjectMeta: metav1.ObjectMeta{
						Name:      dnsHealthCheckProbeName(address.Value, gw.Name, string(listener.Name)),
						Namespace: gw.Namespace,
						Labels:    commonDNSRecordLabels(client.ObjectKeyFromObject(gw), client.ObjectKeyFromObject(dnsPolicy)),
					},
					Spec: v1alpha1.DNSHealthCheckProbeSpec{
						Port:                     *port,
						Host:                     string(*listener.Hostname),
						Address:                  address.Value,
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
	}

	return healthChecks, ignoredHosts
}

func dnsHealthCheckProbeName(address, gatewayName, listenerName string) string {
	return fmt.Sprintf("%s-%s", address, dnsRecordName(gatewayName, listenerName))
}
