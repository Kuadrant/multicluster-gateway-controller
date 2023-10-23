//go:build unit

package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

func TestHealthCheckReconcile(t *testing.T) {
	testCases := []struct {
		name string

		spec                 dns.HealthCheckSpec
		endpoint             *v1alpha2.Endpoint
		existingHealthChecks []*mockHealthCheck

		assertion func(dns.HealthCheckResult, error, *v1alpha2.Endpoint, *mockRoute53API) error
	}{
		{
			name: "New health check created",

			spec: dns.HealthCheckSpec{
				Name: "test-health-check",
			},
			endpoint: &v1alpha2.Endpoint{},
			existingHealthChecks: []*mockHealthCheck{
				{
					HealthCheck: &route53.HealthCheck{
						Id: ptrTo("test-0"),
					},
				},
			},

			assertion: func(hcr dns.HealthCheckResult, err error, e *v1alpha2.Endpoint, mra *mockRoute53API) error {
				if hcr.Result != dns.HealthCheckCreated {
					return fmt.Errorf("unexpected result. Expected Created, but got %s", hcr.Result)
				}
				if err != nil {
					return fmt.Errorf("unexpected error %v", err)
				}
				healthCheckId, ok := e.GetProviderSpecific(ProviderSpecificHealthCheckID)
				if !ok {
					return fmt.Errorf("expected provider specific to be set")
				}

				if healthCheckId != "test-1" {
					return fmt.Errorf("expected health check id to be test-1, but got %s", healthCheckId)
				}
				if len(mra.healthChecks) != 2 {
					return fmt.Errorf("expected 2 health checks after update, got %v", mra.healthChecks)
				}

				return nil
			},
		},
		{
			name: "Update existing health check",

			spec: dns.HealthCheckSpec{
				Port: ptrTo(int64(443)),
				Id:   "test-0",
				Path: "/",
			},
			endpoint: &v1alpha2.Endpoint{
				ProviderSpecific: v1alpha2.ProviderSpecific{
					{
						Name:  ProviderSpecificHealthCheckID,
						Value: "test-0",
					},
				},
			},
			existingHealthChecks: []*mockHealthCheck{
				{
					HealthCheck: &route53.HealthCheck{
						Id: ptrTo("test-0"),
						HealthCheckConfig: &route53.HealthCheckConfig{
							Port: ptrTo(int64(8000)),
						},
					},
				},
				{
					HealthCheck: &route53.HealthCheck{
						Id: ptrTo("test-1"),
					},
				},
			},
			assertion: func(hcr dns.HealthCheckResult, err error, e *v1alpha2.Endpoint, mra *mockRoute53API) error {
				if err != nil {
					return fmt.Errorf("unexpected errror %v", err)
				}
				if hcr.Result != dns.HealthCheckUpdated {
					return fmt.Errorf("unexpected result. Expected Updated, got %v", hcr)

				}
				if len(mra.healthChecks) != 2 {
					return fmt.Errorf("expected 2 health checks after update, got %v", mra.healthChecks)
				}

				return nil
			},
		},
		{
			name: "Existing health check not updated",

			spec: dns.HealthCheckSpec{
				Port: ptrTo(int64(443)),
				Id:   "test-0",
				Path: "/",
			},
			endpoint: &v1alpha2.Endpoint{
				DNSName:       "test.example.com",
				Targets:       v1alpha2.Targets{"0.0.0.0"},
				SetIdentifier: "0.0.0.0",
				ProviderSpecific: v1alpha2.ProviderSpecific{
					{
						Name:  ProviderSpecificHealthCheckID,
						Value: "test-0",
					},
				},
			},
			existingHealthChecks: []*mockHealthCheck{
				{
					HealthCheck: &route53.HealthCheck{
						Id: ptrTo("test-0"),
						HealthCheckConfig: &route53.HealthCheckConfig{
							Port:                     ptrTo(int64(443)),
							ResourcePath:             ptrTo("/"),
							IPAddress:                ptrTo("0.0.0.0"),
							FullyQualifiedDomainName: ptrTo("test.example.com"),
						},
					},
				},
				{
					HealthCheck: &route53.HealthCheck{
						Id: ptrTo("test-1"),
					},
				},
			},
			assertion: func(hcr dns.HealthCheckResult, err error, e *v1alpha2.Endpoint, mra *mockRoute53API) error {
				if err != nil {
					return fmt.Errorf("unexpected errror %v", err)
				}
				if hcr.Result != dns.HealthCheckNoop {
					return fmt.Errorf("unexpected result. Expected Noop, got %v", hcr)

				}
				if len(mra.healthChecks) != 2 {
					return fmt.Errorf("expected 2 health checks after update, got %v", mra.healthChecks)
				}

				return nil
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := &mockRoute53API{
				healthChecks: testCase.existingHealthChecks,
			}
			reconciler := NewRoute53HealthCheckReconciler(client)

			result, reconcileErr := reconciler.Reconcile(context.TODO(), testCase.spec, testCase.endpoint)
			if testErr := testCase.assertion(result, reconcileErr, testCase.endpoint, client); testErr != nil {
				t.Fatal(testErr)
			}
		})
	}
}

func TestHealthCheckDelete(t *testing.T) {
	testCases := []struct {
		name string

		endpoint             *v1alpha2.Endpoint
		existingHealthChecks []*mockHealthCheck

		assertion func(dns.HealthCheckResult, error, *v1alpha2.Endpoint, *mockRoute53API) error
	}{
		{
			name: "Test case deleted",

			endpoint: &v1alpha2.Endpoint{
				ProviderSpecific: v1alpha2.ProviderSpecific{
					{
						Name:  ProviderSpecificHealthCheckID,
						Value: "test-1",
					},
				},
			},
			existingHealthChecks: []*mockHealthCheck{
				{
					HealthCheck: &route53.HealthCheck{
						Id: ptrTo("test-0"),
					},
				},
				{
					HealthCheck: &route53.HealthCheck{
						Id: ptrTo("test-1"),
					},
				},
			},

			assertion: func(hcr dns.HealthCheckResult, err error, e *v1alpha2.Endpoint, mra *mockRoute53API) error {
				if err != nil {
					return fmt.Errorf("unexpected error %v", err)
				}
				if hcr.Result != dns.HealthCheckDeleted {
					return fmt.Errorf("unexpected result. Expected Deleted, got %s", hcr.Result)
				}
				healthCheckID, ok := e.GetProviderSpecific(ProviderSpecificHealthCheckID)
				if ok {
					return fmt.Errorf("expected provider specific %s to be removed, but got %s", ProviderSpecificHealthCheckID, healthCheckID)
				}
				if len(mra.healthChecks) != 1 {
					return fmt.Errorf("expected number of remaining health checks to be 1 but got %v", mra.healthChecks)
				}

				return nil
			},
		},

		{
			name: "Test case not found",

			endpoint: &v1alpha2.Endpoint{
				ProviderSpecific: v1alpha2.ProviderSpecific{},
			},
			existingHealthChecks: []*mockHealthCheck{
				{
					HealthCheck: &route53.HealthCheck{
						Id: ptrTo("test-0"),
					},
				},
			},

			assertion: func(hcr dns.HealthCheckResult, err error, e *v1alpha2.Endpoint, mra *mockRoute53API) error {
				if err != nil {
					return fmt.Errorf("unexpected error %v", err)
				}
				if hcr.Result != dns.HealthCheckNoop {
					return fmt.Errorf("unexpected result. Expected Noop, got %s", hcr.Result)
				}
				healthCheckID, ok := e.GetProviderSpecific(ProviderSpecificHealthCheckID)
				if ok {
					return fmt.Errorf("expected provider specific %s to be removed, but got %s", ProviderSpecificHealthCheckID, healthCheckID)
				}
				if len(mra.healthChecks) != 1 {
					return fmt.Errorf("expected number of remaining health checks to be 1 but got %v", mra.healthChecks)
				}

				return nil
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := &mockRoute53API{
				healthChecks: testCase.existingHealthChecks,
			}
			reconciler := NewRoute53HealthCheckReconciler(client)

			result, reconcileErr := reconciler.Delete(context.TODO(), testCase.endpoint)
			if testErr := testCase.assertion(result, reconcileErr, testCase.endpoint, client); testErr != nil {
				t.Fatal(testErr)
			}
		})
	}
}

type mockRoute53API struct {
	unimplementedRoute53
	healthChecks []*mockHealthCheck
}

type mockHealthCheck struct {
	*route53.HealthCheck
	tags []*route53.Tag
}

func (m *mockRoute53API) ChangeTagsForResourceWithContext(_ context.Context, i *route53.ChangeTagsForResourceInput, _ ...request.Option) (*route53.ChangeTagsForResourceOutput, error) {
	healthCheck, ok := slice.Find(m.healthChecks, func(h *mockHealthCheck) bool {
		return *h.Id == *i.ResourceId
	})
	if !ok {
		return &route53.ChangeTagsForResourceOutput{}, nil
	}

	healthCheck.tags = append(healthCheck.tags, i.AddTags...)
	return &route53.ChangeTagsForResourceOutput{}, nil
}

func (m *mockRoute53API) CreateHealthCheck(input *route53.CreateHealthCheckInput) (*route53.CreateHealthCheckOutput, error) {
	return m.CreateHealthCheckWithContext(context.TODO(), input)
}

func (m *mockRoute53API) CreateHealthCheckWithContext(_ context.Context, i *route53.CreateHealthCheckInput, _ ...request.Option) (*route53.CreateHealthCheckOutput, error) {
	healthCheck := &route53.HealthCheck{
		HealthCheckConfig: i.HealthCheckConfig,
		Id:                ptrTo(fmt.Sprintf("test-%d", len(m.healthChecks))),
	}

	m.healthChecks = append(m.healthChecks, &mockHealthCheck{
		HealthCheck: healthCheck,
		tags:        []*route53.Tag{},
	})

	return &route53.CreateHealthCheckOutput{
		HealthCheck: healthCheck,
	}, nil
}

func (m *mockRoute53API) DeleteHealthCheck(i *route53.DeleteHealthCheckInput) (*route53.DeleteHealthCheckOutput, error) {
	return m.DeleteHealthCheckWithContext(context.TODO(), i)
}

func (m *mockRoute53API) DeleteHealthCheckWithContext(_ context.Context, i *route53.DeleteHealthCheckInput, _ ...request.Option) (*route53.DeleteHealthCheckOutput, error) {
	m.healthChecks = slice.Filter(m.healthChecks, func(h *mockHealthCheck) bool {
		return *h.Id == *i.HealthCheckId
	})
	return &route53.DeleteHealthCheckOutput{}, nil
}

func (m *mockRoute53API) GetHealthCheck(i *route53.GetHealthCheckInput) (*route53.GetHealthCheckOutput, error) {
	return m.GetHealthCheckWithContext(context.TODO(), i)
}

func (m *mockRoute53API) GetHealthCheckWithContext(_ context.Context, i *route53.GetHealthCheckInput, _ ...request.Option) (*route53.GetHealthCheckOutput, error) {
	healthCheck, ok := slice.Find(m.healthChecks, func(hc *mockHealthCheck) bool {
		return *i.HealthCheckId == *hc.Id
	})

	var result *route53.HealthCheck = nil
	if ok {
		result = healthCheck.HealthCheck
	}

	return &route53.GetHealthCheckOutput{
		HealthCheck: result,
	}, nil
}

func (m *mockRoute53API) UpdateHealthCheck(i *route53.UpdateHealthCheckInput) (*route53.UpdateHealthCheckOutput, error) {
	return m.UpdateHealthCheckWithContext(context.TODO(), i)
}

// UpdateHealthCheckWithContext implements route53iface.Route53API
func (m *mockRoute53API) UpdateHealthCheckWithContext(_ context.Context, i *route53.UpdateHealthCheckInput, _ ...request.Option) (*route53.UpdateHealthCheckOutput, error) {
	healthCheck, ok := slice.Find(m.healthChecks, func(h *mockHealthCheck) bool {
		return *h.Id == *i.HealthCheckId
	})
	if !ok {
		return &route53.UpdateHealthCheckOutput{}, nil
	}

	healthCheck.HealthCheckConfig.FailureThreshold = i.FailureThreshold
	healthCheck.HealthCheckConfig.Port = i.Port
	healthCheck.HealthCheckConfig.IPAddress = i.IPAddress

	return &route53.UpdateHealthCheckOutput{HealthCheck: healthCheck.HealthCheck}, nil
}

var _ route53iface.Route53API = &mockRoute53API{}

func ptrTo[T any](value T) *T {
	return &value
}
