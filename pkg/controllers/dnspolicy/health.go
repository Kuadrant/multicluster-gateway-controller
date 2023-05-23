package dnspolicy

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

// healthChecksConfig represents the user configuration for the health checks
type healthChecksConfig struct {
	Endpoint         string
	Port             *int64
	FailureThreshold *int64
	Protocol         *dns.HealthCheckProtocol
}

func (r *DNSPolicyReconciler) ReconcileHealthChecks(ctx context.Context, log logr.Logger, policy *v1alpha1.DNSPolicy) error {
	// Get the associated DNSRecord
	dnsRecords, err := r.getDNSRecords(ctx, policy)
	if err != nil {
		return err
	}
	if len(dnsRecords) == 0 {
		log.Info("No DNSRecord found matching policy", "policy", policy)
		return nil
	}

	// Get the configuration for the health checks. If no configuration is
	// set, ensure that the health checks are deleted
	config := getHealthChecksConfig(policy)

	var allResults []dns.HealthCheckResult

	for _, dnsRecord := range dnsRecords {
		// Keep a copy of the DNSRecord to check if it needs to be updated after
		// reconciling the health checks
		dnsRecordOriginal := dnsRecord.DeepCopy()

		var results []dns.HealthCheckResult

		if config == nil {
			log.V(3).Info("No health check for DNSPolicy, ensuring deletion")
			results, err = r.reconcileHealthCheckDeletion(ctx, dnsRecord, policy)
			if err != nil {
				return err
			}
		} else {
			log.Info("reconciling health checks")
			results, err = r.reconcileHealthChecks(ctx, log, dnsRecord, policy, config)
			if err != nil {
				return err
			}
		}

		allResults = append(allResults, results...)

		if err = r.updateDNSRecord(ctx, dnsRecordOriginal, dnsRecord); err != nil {
			return err
		}
	}

	result := r.reconcileHealthCheckStatus(allResults, policy)
	log.Info("reconciling health check status", "result", result, "policy status", policy.Status)
	return result
}

func (r *DNSPolicyReconciler) reconcileHealthChecks(ctx context.Context, log logr.Logger, dnsRecord *v1alpha1.DNSRecord, policy *v1alpha1.DNSPolicy, config *healthChecksConfig) ([]dns.HealthCheckResult, error) {
	healthCheckReconciler := r.DNSProvider.HealthCheckReconciler()

	results := []dns.HealthCheckResult{}

	for _, dnsEndpoint := range dnsRecord.Spec.Endpoints {
		log.Info("reconciling DNS Record health checks", "endpoint", dnsEndpoint)
		if a, ok := dnsEndpoint.GetAddress(); !ok {
			log.Info("address not ok", "address", a)
			continue
		}

		endpointId, err := idForEndpoint(dnsRecord, dnsEndpoint)
		if err != nil {
			return nil, err
		}

		spec := dns.HealthCheckSpec{
			Id:               endpointId,
			Name:             fmt.Sprintf("%s-%s", dnsEndpoint.DNSName, dnsEndpoint.SetIdentifier),
			Path:             config.Endpoint,
			Port:             config.Port,
			Protocol:         config.Protocol,
			FailureThreshold: config.FailureThreshold,
		}

		result, err := healthCheckReconciler.Reconcile(ctx, spec, dnsEndpoint)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	log.Info("returning health checks", "count", len(results))
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

func (r *DNSPolicyReconciler) reconcileHealthCheckDeletion(ctx context.Context, dnsRecord *v1alpha1.DNSRecord, policy *v1alpha1.DNSPolicy) ([]dns.HealthCheckResult, error) {
	reconciler := r.DNSProvider.HealthCheckReconciler()
	results := []dns.HealthCheckResult{}

	for _, endpoint := range dnsRecord.Spec.Endpoints {
		result, err := reconciler.Delete(ctx, endpoint)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}

func (r *DNSPolicyReconciler) getDNSRecords(ctx context.Context, policy *v1alpha1.DNSPolicy) ([]*v1alpha1.DNSRecord, error) {
	return getDNSRecords(ctx, r.Client, r.HostService, policy)
}

func (r *DNSPolicyReconciler) updateDNSRecord(ctx context.Context, original, updated *v1alpha1.DNSRecord) error {
	if equality.Semantic.DeepEqual(original, updated) {
		return nil
	}

	return r.Client.Update(ctx, updated)
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
