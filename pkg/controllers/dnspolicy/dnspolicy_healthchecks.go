package dnspolicy

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/kuadrant/kuadrant-operator/pkg/common"
	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

var (
	defaultPort = 443
)

func (r *DNSPolicyReconciler) reconcileHealthChecks(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
	log := crlog.FromContext(ctx)

	log.V(3).Info("reconciling health checks")
	for _, gw := range append(gwDiffObj.GatewaysWithValidPolicyRef, gwDiffObj.GatewaysMissingPolicyRef...) {
		log.V(3).Info("reconciling probes", "gateway", gw.Name)
		expectedProbes, err := r.expectedProbesForGateway(ctx, gw, dnsPolicy)
		if err != nil {
			return fmt.Errorf("error generating probes for gateway %v: %w", gw.Gateway.Name, err)
		}
		if err := r.createOrUpdateProbes(ctx, expectedProbes); err != nil {
			return fmt.Errorf("error creating and updating expected proves for gateway %v: %w", gw.Gateway.Name, err)
		}
		if err := r.deleteUnexpectedGatewayProbes(ctx, expectedProbes, gw, dnsPolicy); err != nil {
			return fmt.Errorf("error removing unexpected probes for gateway %v: %w", gw.Gateway.Name, err)
		}

	}

	for _, gw := range gwDiffObj.GatewaysWithInvalidPolicyRef {
		log.V(3).Info("deleting probes", "gateway", gw.Gateway.Name)
		if err := r.deleteUnexpectedGatewayProbes(ctx, []*v1alpha1.DNSHealthCheckProbe{}, gw, dnsPolicy); err != nil {
			return fmt.Errorf("error deleting probes for gw %v: %w", gw.Gateway.Name, err)
		}
	}

	return nil
}

func (r *DNSPolicyReconciler) createOrUpdateProbes(ctx context.Context, expectedProbes []*v1alpha1.DNSHealthCheckProbe) error {
	//create or update all expected probes
	for _, hcProbe := range expectedProbes {
		p := &v1alpha1.DNSHealthCheckProbe{}
		if err := r.Client().Get(ctx, client.ObjectKeyFromObject(hcProbe), p); errors.IsNotFound(err) {
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

func (r *DNSPolicyReconciler) deleteUnexpectedGatewayProbes(ctx context.Context, expectedProbes []*v1alpha1.DNSHealthCheckProbe, gw common.GatewayWrapper, dnsPolicy *v1alpha1.DNSPolicy) error {
	// remove any probes for this gateway and DNS Policy that are no longer expected
	existingProbes := &v1alpha1.DNSHealthCheckProbeList{}
	dnsLabels := commonDNSRecordLabels(client.ObjectKeyFromObject(gw), client.ObjectKeyFromObject(dnsPolicy))
	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(dnsLabels)}
	if err := r.Client().List(ctx, existingProbes, listOptions); client.IgnoreNotFound(err) != nil {
		return err
	} else {
		for _, p := range existingProbes.Items {
			if !slice.Contains(expectedProbes, func(expectedProbe *v1alpha1.DNSHealthCheckProbe) bool {
				return expectedProbe.Name == p.Name && expectedProbe.Namespace == p.Namespace
			}) {
				if err := r.Client().Delete(ctx, &p); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (r *DNSPolicyReconciler) expectedProbesForGateway(ctx context.Context, gw common.GatewayWrapper, dnsPolicy *v1alpha1.DNSPolicy) ([]*v1alpha1.DNSHealthCheckProbe, error) {
	log := crlog.FromContext(ctx)
	var healthChecks []*v1alpha1.DNSHealthCheckProbe
	if dnsPolicy.Spec.HealthCheck == nil {
		log.V(3).Info("DNS Policy has no defined health check")
		return nil, nil
	}
	ipPattern := `\b(?:\d{1,3}\.){3}\d{1,3}\b`
	re := regexp.MustCompile(ipPattern)

	interval := metav1.Duration{Duration: 60 * time.Second}
	if dnsPolicy.Spec.HealthCheck.Interval != nil {
		interval = *dnsPolicy.Spec.HealthCheck.Interval
	}

	for _, address := range gw.Status.Addresses {
		port := dnsPolicy.Spec.HealthCheck.Port
		if port == nil {
			port = &defaultPort
		}

		matches := re.FindAllString(address.Value, -1)
		if len(matches) != 1 {
			log.V(3).Info("Found more or less than 1 ip address")
			continue
		}

		for _, listener := range gw.Spec.Listeners {
			if strings.Contains(string(*listener.Hostname), "*") {
				continue
			}
			log.V(1).Info("reconcileHealthChecks: adding health check for target", "target", address.Value)
			healthCheck := &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-%s", matches[0], dnsPolicy.Name, listener.Name),
					Namespace: gw.Namespace,
					Labels:    commonDNSRecordLabels(client.ObjectKeyFromObject(gw), client.ObjectKeyFromObject(dnsPolicy)),
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					Port:                     *port,
					Host:                     string(*listener.Hostname),
					Address:                  matches[0],
					Path:                     dnsPolicy.Spec.HealthCheck.Endpoint,
					Protocol:                 *dnsPolicy.Spec.HealthCheck.Protocol,
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

	return healthChecks, nil
}
