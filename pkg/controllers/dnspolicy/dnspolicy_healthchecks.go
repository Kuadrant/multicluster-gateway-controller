package dnspolicy

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
)

// healthChecksConfig represents the user configuration for the health checks
type healthChecksConfig struct {
	Endpoint         string
	Port             *int64
	FailureThreshold *int64
	Protocol         *dns.HealthCheckProtocol
}

func (r *DNSPolicyReconciler) reconcileHealthChecks(ctx context.Context, dnsPolicy *v1alpha1.DNSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
	log := crlog.FromContext(ctx)

	// Delete Health checks for each gateway no longer referred by this policy
	for _, gw := range gwDiffObj.GatewaysWithInvalidPolicyRef {
		log.V(1).Info("reconcileHealthChecks: gateway with invalid policy ref", "key", gw.Key())
		_, err := r.reconcileGatewayHealthChecks(ctx, gw.Gateway, nil)
		if err != nil {
			return err
		}
	}

	healthCheckConfig := getHealthChecksConfig(dnsPolicy)
	allResults := []dns.HealthCheckResult{}

	// Reconcile Health checks for each gateway directly referred by the policy (existing and new)
	for _, gw := range append(gwDiffObj.GatewaysWithValidPolicyRef, gwDiffObj.GatewaysMissingPolicyRef...) {
		log.V(1).Info("reconcileHealthChecks: gateway with valid and missing policy ref", "key", gw.Key())
		results, err := r.reconcileGatewayHealthChecks(ctx, gw.Gateway, healthCheckConfig)
		if err != nil {
			return err
		}
		allResults = append(allResults, results...)
	}

	return r.reconcileHealthCheckStatus(allResults, dnsPolicy)
}

func (r *DNSPolicyReconciler) reconcileGatewayHealthChecks(ctx context.Context, gateway *gatewayv1beta1.Gateway, config *healthChecksConfig) ([]dns.HealthCheckResult, error) {
	allResults := []dns.HealthCheckResult{}

	gatewayAccessor := traffic.NewGateway(gateway)
	managedHosts, err := r.dnsHelper.getManagedHosts(ctx, gatewayAccessor)
	if err != nil {
		return allResults, err
	}

	for _, mh := range managedHosts {
		if mh.DnsRecord == nil {
			continue
		}

		// Keep a copy of the DNSRecord to check if it needs to be updated after
		// reconciling the health checks
		dnsRecordOriginal := mh.DnsRecord.DeepCopy()

		results, err := r.reconcileDNSRecordHealthChecks(ctx, mh.DnsRecord, config)
		if err != nil {
			return allResults, err
		}

		allResults = append(allResults, results...)

		if !equality.Semantic.DeepEqual(dnsRecordOriginal, mh.DnsRecord) {
			err = r.Client().Update(ctx, mh.DnsRecord)
			if err != nil {
				return allResults, err
			}
		}
	}
	return allResults, nil
}

func (r *DNSPolicyReconciler) reconcileDNSRecordHealthChecks(ctx context.Context, dnsRecord *v1alpha1.DNSRecord, config *healthChecksConfig) ([]dns.HealthCheckResult, error) {

	managedzone, err := r.dnsHelper.getDNSRecordManagedZone(ctx, dnsRecord)
	if err != nil {
		return nil, err
	}
	dnsProvider, err := r.DNSProvider(ctx, managedzone)
	if err != nil {
		return nil, err
	}

	healthCheckReconciler := dnsProvider.HealthCheckReconciler()
	results := []dns.HealthCheckResult{}

	for _, endpoint := range dnsRecord.Spec.Endpoints {
		if config == nil {
			result, err := healthCheckReconciler.Delete(ctx, endpoint)
			if err != nil {
				return nil, err
			}
			results = append(results, result)
			continue
		}
		if _, ok := endpoint.GetAddress(); !ok {
			continue
		}

		endpointId, err := idForEndpoint(dnsRecord, endpoint)
		if err != nil {
			return nil, err
		}

		spec := dns.HealthCheckSpec{
			Id:               endpointId,
			Name:             fmt.Sprintf("%s-%s", endpoint.DNSName, endpoint.SetIdentifier),
			Path:             config.Endpoint,
			Port:             config.Port,
			Protocol:         config.Protocol,
			FailureThreshold: config.FailureThreshold,
		}

		result, err := healthCheckReconciler.Reconcile(ctx, spec, endpoint)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

func (r *DNSPolicyReconciler) reconcileHealthCheckStatus(results []dns.HealthCheckResult, policy *v1alpha1.DNSPolicy) error {
	for _, result := range results {
		if result.Result == dns.HealthCheckNoop {
			continue
		}

		//reset health check status
		policy.Status.HealthCheck = &v1alpha1.HealthCheckStatus{}

		status := metav1.ConditionTrue
		if result.Result == dns.HealthCheckFailed {
			status = metav1.ConditionFalse
		}

		policy.Status.HealthCheck.Conditions = append(policy.Status.HealthCheck.Conditions, metav1.Condition{
			ObservedGeneration: policy.Generation,
			Status:             status,
			Reason:             string(result.Result),
			LastTransitionTime: metav1.Now(),
			Message:            result.Message,
			Type:               string(result.Result),
		})
	}

	return nil
}

func getHealthChecksConfig(policy *v1alpha1.DNSPolicy) *healthChecksConfig {
	if policy.Spec.HealthCheck == nil {
		return nil
	}

	return &healthChecksConfig{
		Endpoint:         policy.Spec.HealthCheck.Endpoint,
		Port:             valueAs(toInt64, policy.Spec.HealthCheck.Port),
		FailureThreshold: valueAs(toInt64, policy.Spec.HealthCheck.FailureThreshold),
		Protocol:         (*dns.HealthCheckProtocol)(policy.Spec.HealthCheck.Protocol),
	}
}

func valueAs[T, R any](f func(T) R, original *T) *R {
	if original == nil {
		return nil
	}

	value := f(*original)
	return &value
}

func toInt64(original int) int64 {
	return int64(original)
}

// idForEndpoint returns a unique identifier for an endpoint
func idForEndpoint(dnsRecord *v1alpha1.DNSRecord, endpoint *v1alpha1.Endpoint) (string, error) {
	hash := md5.New()
	if _, err := io.WriteString(hash, fmt.Sprintf("%s/%s@%s", dnsRecord.Name, endpoint.SetIdentifier, endpoint.DNSName)); err != nil {
		return "", fmt.Errorf("unexpected error creating ID for endpoint %s", endpoint.SetIdentifier)
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
