package dns

import (
	"context"
	"reflect"

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

type CachedHealthCheckReconciler struct {
	reconciler HealthCheckReconciler
	provider   Provider

	cache map[string]HealthCheckSpec
}

var _ HealthCheckReconciler = &CachedHealthCheckReconciler{}

func NewCachedHealthCheckReconciler(provider Provider, reconciler HealthCheckReconciler) *CachedHealthCheckReconciler {
	return &CachedHealthCheckReconciler{
		reconciler: reconciler,
		provider:   provider,
		cache:      map[string]HealthCheckSpec{},
	}
}

// Delete implements HealthCheckReconciler
func (r *CachedHealthCheckReconciler) Delete(ctx context.Context, endpoint *v1alpha1.Endpoint) (HealthCheckResult, error) {
	id, ok := r.getHealthCheckID(endpoint)
	if !ok {
		return NewHealthCheckResult(HealthCheckNoop, ""), nil
	}

	defer delete(r.cache, id)
	return r.reconciler.Delete(ctx, endpoint)
}

// Reconcile implements HealthCheckReconciler
func (r *CachedHealthCheckReconciler) Reconcile(ctx context.Context, spec HealthCheckSpec, endpoint *v1alpha1.Endpoint) (HealthCheckResult, error) {
	id, ok := r.getHealthCheckID(endpoint)
	if !ok {
		return r.reconciler.Reconcile(ctx, spec, endpoint)
	}

	// Update the cache with the new spec
	defer func() {
		r.cache[id] = spec
	}()

	// If the health heck is not cached, delegate the reconciliation
	existingSpec, ok := r.cache[id]
	if !ok {
		return r.reconciler.Reconcile(ctx, spec, endpoint)
	}

	// If the spec is unchanged, return Noop
	if reflect.DeepEqual(spec, existingSpec) {
		return NewHealthCheckResult(HealthCheckNoop, "Spec unchanged"), nil
	}

	// Otherwise, delegate the reconciliation
	return r.reconciler.Reconcile(ctx, spec, endpoint)
}

func (r *CachedHealthCheckReconciler) getHealthCheckID(endpoint *v1alpha1.Endpoint) (string, bool) {
	return endpoint.GetProviderSpecific(r.provider.ProviderSpecific().HealthCheckID)
}
