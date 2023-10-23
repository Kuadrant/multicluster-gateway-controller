package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/aws/aws-sdk-go/service/route53/route53iface"
	"github.com/rs/xid"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	idTag = "kuadrant.dev/healthcheck"

	defaultHealthCheckPath             = "/"
	defaultHealthCheckPort             = 80
	defaultHealthCheckFailureThreshold = 3
)

var (
	callerReference func(id string) *string
)

type Route53HealthCheckReconciler struct {
	client route53iface.Route53API
}

var _ dns.HealthCheckReconciler = &Route53HealthCheckReconciler{}

func NewRoute53HealthCheckReconciler(client route53iface.Route53API) *Route53HealthCheckReconciler {
	return &Route53HealthCheckReconciler{
		client: client,
	}
}

func (r *Route53HealthCheckReconciler) Reconcile(ctx context.Context, spec dns.HealthCheckSpec, endpoint *v1alpha2.Endpoint) (dns.HealthCheckResult, error) {
	healthCheck, exists, err := r.findHealthCheck(ctx, endpoint)
	if err != nil {
		return dns.HealthCheckResult{}, err
	}

	defer func() {
		if healthCheck != nil {
			endpoint.SetProviderSpecific(ProviderSpecificHealthCheckID, *healthCheck.Id)
		}
	}()

	if exists {
		status, err := r.updateHealthCheck(ctx, spec, endpoint, healthCheck)
		if err != nil {
			return dns.HealthCheckResult{}, err
		}

		return dns.NewHealthCheckResult(status, ""), nil
	}

	healthCheck, err = r.createHealthCheck(ctx, spec, endpoint)
	if err != nil {
		return dns.HealthCheckResult{}, err
	}

	return dns.NewHealthCheckResult(dns.HealthCheckCreated, fmt.Sprintf("Created health check with ID %s", *healthCheck.Id)), nil
}

func (r *Route53HealthCheckReconciler) Delete(ctx context.Context, endpoint *v1alpha2.Endpoint) (dns.HealthCheckResult, error) {
	healthCheck, found, err := r.findHealthCheck(ctx, endpoint)
	if err != nil {
		return dns.HealthCheckResult{}, err
	}
	if !found {
		return dns.NewHealthCheckResult(dns.HealthCheckNoop, ""), nil
	}

	_, err = r.client.DeleteHealthCheckWithContext(ctx, &route53.DeleteHealthCheckInput{
		HealthCheckId: healthCheck.Id,
	})

	if err != nil {
		return dns.HealthCheckResult{}, err
	}

	endpoint.DeleteProviderSpecific(ProviderSpecificHealthCheckID)
	return dns.NewHealthCheckResult(dns.HealthCheckDeleted, ""), nil
}

func (c *Route53HealthCheckReconciler) findHealthCheck(ctx context.Context, endpoint *v1alpha2.Endpoint) (*route53.HealthCheck, bool, error) {
	id, hasId := getHealthCheckId(endpoint)
	if !hasId {
		return nil, false, nil
	}

	response, err := c.client.GetHealthCheckWithContext(ctx, &route53.GetHealthCheckInput{
		HealthCheckId: &id,
	})
	if err != nil {
		return nil, false, err
	}

	return response.HealthCheck, true, nil

}

func (c *Route53HealthCheckReconciler) createHealthCheck(ctx context.Context, spec dns.HealthCheckSpec, endpoint *v1alpha2.Endpoint) (*route53.HealthCheck, error) {
	address, _ := endpoint.GetAddress()
	host := endpoint.DNSName

	// Create the health check
	output, err := c.client.CreateHealthCheck(&route53.CreateHealthCheckInput{

		CallerReference: callerReference(spec.Id),

		HealthCheckConfig: &route53.HealthCheckConfig{
			IPAddress:                &address,
			FullyQualifiedDomainName: &host,
			Port:                     spec.Port,
			ResourcePath:             &spec.Path,
			Type:                     healthCheckType(spec.Protocol),
			FailureThreshold:         spec.FailureThreshold,
		},
	})
	if err != nil {
		return nil, err
	}

	// Add the tag to identify it
	_, err = c.client.ChangeTagsForResourceWithContext(ctx, &route53.ChangeTagsForResourceInput{
		AddTags: []*route53.Tag{
			{
				Key:   aws.String(idTag),
				Value: aws.String(spec.Id),
			},
			{
				Key:   aws.String("Name"),
				Value: &spec.Name,
			},
		},
		ResourceId:   output.HealthCheck.Id,
		ResourceType: aws.String(route53.TagResourceTypeHealthcheck),
	})
	if err != nil {
		return nil, err
	}

	return output.HealthCheck, nil
}

func (r *Route53HealthCheckReconciler) updateHealthCheck(ctx context.Context, spec dns.HealthCheckSpec, endpoint *v1alpha2.Endpoint, healthCheck *route53.HealthCheck) (dns.HealthCheckReconciliationResult, error) {
	diff := healthCheckDiff(healthCheck, spec, endpoint)
	if diff == nil {
		return dns.HealthCheckNoop, nil
	}

	log.Log.Info("Updating health check", "diff", *diff)

	_, err := r.client.UpdateHealthCheckWithContext(ctx, diff)
	if err != nil {
		return dns.HealthCheckFailed, err
	}

	return dns.HealthCheckUpdated, nil
}

// healthCheckDiff creates a `UpdateHealthCheckInput` object with the fields to
// update on healthCheck based on the given spec.
// If the health check matches the spec, returns `nil`
func healthCheckDiff(healthCheck *route53.HealthCheck, spec dns.HealthCheckSpec, endpoint *v1alpha2.Endpoint) *route53.UpdateHealthCheckInput {
	var result *route53.UpdateHealthCheckInput

	// "Lazily" set the value for result only once and only when there is
	// a change, to ensure that it's nil if there's no change
	diff := func() *route53.UpdateHealthCheckInput {
		if result == nil {
			result = &route53.UpdateHealthCheckInput{
				HealthCheckId: healthCheck.Id,
			}
		}

		return result
	}

	if !valuesEqual(&endpoint.DNSName, healthCheck.HealthCheckConfig.FullyQualifiedDomainName) {
		diff().FullyQualifiedDomainName = &endpoint.DNSName
	}

	address, _ := endpoint.GetAddress()
	if !valuesEqual(&address, healthCheck.HealthCheckConfig.IPAddress) {
		diff().IPAddress = &address
	}
	if !valuesEqualWithDefault(&spec.Path, healthCheck.HealthCheckConfig.ResourcePath, defaultHealthCheckPath) {
		diff().ResourcePath = &spec.Path
	}
	if !valuesEqualWithDefault(spec.Port, healthCheck.HealthCheckConfig.Port, defaultHealthCheckPort) {
		diff().Port = spec.Port
	}
	if !valuesEqualWithDefault(spec.FailureThreshold, healthCheck.HealthCheckConfig.FailureThreshold, defaultHealthCheckFailureThreshold) {
		diff().FailureThreshold = spec.FailureThreshold
	}

	return result
}

func init() {
	sid := xid.New()
	callerReference = func(s string) *string {
		return aws.String(fmt.Sprintf("%s.%s", s, sid))
	}
}

func healthCheckType(protocol *dns.HealthCheckProtocol) *string {
	if protocol == nil {
		return nil
	}

	switch *protocol {
	case dns.HealthCheckProtocolHTTP:
		return aws.String(route53.HealthCheckTypeHttp)

	case dns.HealthCheckProtocolHTTPS:
		return aws.String(route53.HealthCheckTypeHttps)
	}

	return nil
}

func valuesEqual[T comparable](ptr1, ptr2 *T) bool {
	if (ptr1 == nil && ptr2 != nil) || (ptr1 != nil && ptr2 == nil) {
		return false
	}
	if ptr1 == nil && ptr2 == nil {
		return true
	}

	return *ptr1 == *ptr2
}

func valuesEqualWithDefault[T comparable](ptr1, ptr2 *T, defaultValue T) bool {
	value1 := defaultValue
	if ptr1 != nil {
		value1 = *ptr1
	}

	value2 := defaultValue
	if ptr2 != nil {
		value2 = *ptr2
	}

	return value1 == value2
}

func getHealthCheckId(endpoint *v1alpha2.Endpoint) (string, bool) {
	return endpoint.GetProviderSpecific(ProviderSpecificHealthCheckID)
}
