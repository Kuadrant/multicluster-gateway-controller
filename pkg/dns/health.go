package dns

import (
	"context"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
)

type HealthCheckReconciler interface {
	Reconcile(ctx context.Context, spec HealthCheckSpec, endpoint *v1alpha1.Endpoint) (HealthCheckResult, error)

	Delete(ctx context.Context, endpoint *v1alpha1.Endpoint) (HealthCheckResult, error)
}

type HealthCheckSpec struct {
	Id               string
	Name             string
	Port             *int64
	FailureThreshold *int64
	Protocol         *HealthCheckProtocol

	Path string
}

type HealthCheckResult struct {
	Result  HealthCheckReconciliationResult
	Message string
}

func NewHealthCheckResult(result HealthCheckReconciliationResult, message string) HealthCheckResult {
	return HealthCheckResult{
		Result:  result,
		Message: message,
	}
}

type HealthCheckReconciliationResult string

const (
	HealthCheckCreated HealthCheckReconciliationResult = "Created"
	HealthCheckUpdated HealthCheckReconciliationResult = "Updated"
	HealthCheckDeleted HealthCheckReconciliationResult = "Deleted"
	HealthCheckNoop    HealthCheckReconciliationResult = "Noop"
	HealthCheckFailed  HealthCheckReconciliationResult = "Failed"
)

type HealthCheckProtocol string

const HealthCheckProtocolHTTP HealthCheckProtocol = "HTTP"
const HealthCheckProtocolHTTPS HealthCheckProtocol = "HTTPS"

type FakeHealthCheckReconciler struct{}

func (*FakeHealthCheckReconciler) Reconcile(ctx context.Context, _ HealthCheckSpec, _ *v1alpha1.Endpoint) (HealthCheckResult, error) {
	return HealthCheckResult{HealthCheckNoop, ""}, nil
}

func (*FakeHealthCheckReconciler) Delete(ctx context.Context, _ *v1alpha1.Endpoint) (HealthCheckResult, error) {
	return HealthCheckResult{HealthCheckDeleted, ""}, nil
}

var _ HealthCheckReconciler = &FakeHealthCheckReconciler{}
