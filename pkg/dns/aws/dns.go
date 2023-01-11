/*
Copyright 2022 The MultiCluster Traffic Controller Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package aws

import (
	"fmt"
	"strconv"

	"github.com/go-logr/logr"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"

	"sigs.k8s.io/controller-runtime/pkg/log"

	kerrors "k8s.io/apimachinery/pkg/util/errors"

	v1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1"
)

const (
	// chinaRoute53Endpoint is the Route 53 service endpoint used for AWS China regions.
	chinaRoute53Endpoint = "https://route53.amazonaws.com.cn"

	ProviderSpecificEvaluateTargetHealth       = "aws/evaluate-target-health"
	ProviderSpecificWeight                     = "aws/weight"
	ProviderSpecificRegion                     = "aws/region"
	ProviderSpecificFailover                   = "aws/failover"
	ProviderSpecificGeolocationContinentCode   = "aws/geolocation-continent-code"
	ProviderSpecificGeolocationCountryCode     = "aws/geolocation-country-code"
	ProviderSpecificGeolocationSubdivisionCode = "aws/geolocation-subdivision-code"
	ProviderSpecificMultiValueAnswer           = "aws/multi-value-answer"
	ProviderSpecificHealthCheckID              = "aws/health-check-id"
)

// Inspired by https://github.com/openshift/cluster-ingress-operator/blob/master/pkg/dns/aws/dns.go
type Provider struct {
	route53 *InstrumentedRoute53
	//healthCheckReconciler *Route53HealthCheckReconciler
	config Config
	logger logr.Logger
}

// Config is the necessary input to configure the manager.
type Config struct {
	// Region is the AWS region ELBs are created in.
	Region string
}

func NewProvider(config Config) (*Provider, error) {
	var region string
	if len(config.Region) > 0 {
		region = config.Region
	}

	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		return nil, fmt.Errorf("couldn't create AWS client session: %v", err)
	}

	r53Config := aws.NewConfig()

	// If the region is in aws china, cn-north-1 or cn-northwest-1, we should:
	// 1. hard code route53 api endpoint to https://route53.amazonaws.com.cn and region to "cn-northwest-1"
	//    as route53 is not GA in AWS China and aws sdk didn't have the endpoint.
	// 2. use the aws china region cn-northwest-1 to setup tagging api correctly instead of "us-east-1"
	switch region {
	case endpoints.CnNorth1RegionID, endpoints.CnNorthwest1RegionID:
		r53Config = r53Config.WithRegion(endpoints.CnNorthwest1RegionID).WithEndpoint(chinaRoute53Endpoint)
	case endpoints.UsGovEast1RegionID, endpoints.UsGovWest1RegionID:
		// Route53 for GovCloud uses the "us-gov-west-1" region id:
		// https://docs.aws.amazon.com/govcloud-us/latest/UserGuide/using-govcloud-endpoints.html
		r53Config = r53Config.WithRegion(endpoints.UsGovWest1RegionID)
	case endpoints.UsIsoEast1RegionID:
		// Do not override the region in C2s
		r53Config = r53Config.WithRegion(region)
	default:
		// Use us-east-1 for Route 53 in AWS Regions other than China or GovCloud Regions.
		// See https://docs.aws.amazon.com/general/latest/gr/r53.html for details.
		r53Config = r53Config.WithRegion(endpoints.UsEast1RegionID)
	}

	p := &Provider{
		route53: &InstrumentedRoute53{route53.New(sess, r53Config)},
		config:  config,
		logger:  log.Log.WithName("aws-route53").WithValues("region", r53Config.Region),
	}
	if err := validateServiceEndpoints(p); err != nil {
		return nil, fmt.Errorf("failed to validate AWS provider service endpoints: %v", err)
	}
	//if p.healthCheckReconciler == nil {
	//	p.healthCheckReconciler = newRoute53HealthCheckReconciler(p.route53, p.logger)
	//}

	return p, nil
}

// validateServiceEndpoints validates that provider clients can communicate with
// associated API endpoints by having each client make a list/describe/get call.
func validateServiceEndpoints(provider *Provider) error {
	var errs []error
	zoneInput := route53.ListHostedZonesInput{MaxItems: aws.String("1")}
	if _, err := provider.route53.ListHostedZones(&zoneInput); err != nil {
		errs = append(errs, fmt.Errorf("failed to list route53 hosted zones: %v", err))
	}
	return kerrors.NewAggregate(errs)
}

type action string

const (
	upsertAction action = "UPSERT"
	deleteAction action = "DELETE"
)

func (p *Provider) Ensure(record *v1.DNSRecord, zone v1.DNSZone) error {
	return p.change(record, zone, upsertAction)
}

func (p *Provider) Delete(record *v1.DNSRecord, zone v1.DNSZone) error {
	return p.change(record, zone, deleteAction)
}

//func (p *Provider) ReconcileHealthCheck(ctx context.Context, hc v1.HealthCheck, endpoint *v1.Endpoint) error {
//
//	return p.healthCheckReconciler.reconcile(ctx, hc, endpoint)
//}
//
//func (p *Provider) DeleteHealthCheck(ctx context.Context, endpoint *v1.Endpoint) error {
//	return p.healthCheckReconciler.deleteHealthCheck(ctx, endpoint)
//}

// change will perform an action on a record.
func (p *Provider) change(record *v1.DNSRecord, zone v1.DNSZone, action action) error {
	// Configure records.
	err := p.updateRecord(record, zone.ID, string(action))
	if err != nil {
		return fmt.Errorf("failed to update record in zone %s: %v", zone.ID, err)
	}
	switch action {
	case upsertAction:
		p.logger.Info("Upserted DNS record", "record", record.Spec, "zone", zone)
	case deleteAction:
		p.logger.Info("Deleted DNS record", "record", record.Spec, "zone", zone)
	}
	return nil
}

func (p *Provider) updateRecord(record *v1.DNSRecord, zoneID, action string) error {
	input := route53.ChangeResourceRecordSetsInput{HostedZoneId: aws.String(zoneID)}

	expectedEndpointsMap := make(map[string]struct{})
	var changes []*route53.Change
	for _, endpoint := range record.Spec.Endpoints {
		expectedEndpointsMap[endpoint.SetID()] = struct{}{}
		change, err := p.changeForEndpoint(endpoint, action)
		if err != nil {
			return err
		}
		changes = append(changes, change)
	}

	// Delete any previously published records that are no longer present in record.Spec.Endpoints
	if action != string(deleteAction) {
		lastPublishedEndpoints, err := p.endpointsFromZoneStatus(record, zoneID)
		if err != nil {
			return err
		}
		for _, endpoint := range lastPublishedEndpoints {
			if _, found := expectedEndpointsMap[endpoint.SetID()]; !found {
				change, err := p.changeForEndpoint(endpoint, string(deleteAction))
				if err != nil {
					return err
				}
				changes = append(changes, change)
			}
		}
	}

	if len(changes) == 0 {
		return nil
	}
	input.ChangeBatch = &route53.ChangeBatch{
		Changes: changes,
	}
	resp, err := p.route53.ChangeResourceRecordSets(&input)
	if err != nil {
		return fmt.Errorf("couldn't update DNS record %s in zone %s: %v", record.Name, zoneID, err)
	}
	p.logger.Info("Updated DNS record", "record", record, "zone", zoneID, "response", resp)
	return nil
}

func (p *Provider) changeForEndpoint(endpoint *v1.Endpoint, action string) (*route53.Change, error) {
	if endpoint.RecordType != string(v1.ARecordType) && endpoint.RecordType != string(v1.CNAMERecordType) {
		return nil, fmt.Errorf("unsupported record type %s", endpoint.RecordType)
	}
	domain, targets := endpoint.DNSName, endpoint.Targets
	if len(domain) == 0 {
		return nil, fmt.Errorf("domain is required")
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("targets is required")
	}

	var resourceRecords []*route53.ResourceRecord
	for _, target := range endpoint.Targets {
		resourceRecords = append(resourceRecords, &route53.ResourceRecord{Value: aws.String(target)})
	}

	resourceRecordSet := &route53.ResourceRecordSet{
		Name:            aws.String(endpoint.DNSName),
		Type:            aws.String(endpoint.RecordType),
		TTL:             aws.Int64(int64(endpoint.RecordTTL)),
		ResourceRecords: resourceRecords,
	}

	if endpoint.SetIdentifier != "" {
		resourceRecordSet.SetIdentifier = aws.String(endpoint.SetIdentifier)
	}
	if prop, ok := endpoint.GetProviderSpecificProperty(ProviderSpecificWeight); ok {
		weight, err := strconv.ParseInt(prop.Value, 10, 64)
		if err != nil {
			p.logger.Error(err, "Failed parsing value, using weight of 0", "weight", ProviderSpecificWeight, "value", prop.Value)
			weight = 0
		}
		resourceRecordSet.Weight = aws.Int64(weight)
	}
	if prop, ok := endpoint.GetProviderSpecificProperty(ProviderSpecificRegion); ok {
		resourceRecordSet.Region = aws.String(prop.Value)
	}
	if prop, ok := endpoint.GetProviderSpecificProperty(ProviderSpecificFailover); ok {
		resourceRecordSet.Failover = aws.String(prop.Value)
	}
	if _, ok := endpoint.GetProviderSpecificProperty(ProviderSpecificMultiValueAnswer); ok {
		resourceRecordSet.MultiValueAnswer = aws.Bool(true)
	}

	var geolocation = &route53.GeoLocation{}
	useGeolocation := false
	if prop, ok := endpoint.GetProviderSpecificProperty(ProviderSpecificGeolocationContinentCode); ok {
		geolocation.ContinentCode = aws.String(prop.Value)
		useGeolocation = true
	} else {
		if prop, ok := endpoint.GetProviderSpecificProperty(ProviderSpecificGeolocationCountryCode); ok {
			geolocation.CountryCode = aws.String(prop.Value)
			useGeolocation = true
		}
		if prop, ok := endpoint.GetProviderSpecificProperty(ProviderSpecificGeolocationSubdivisionCode); ok {
			geolocation.SubdivisionCode = aws.String(prop.Value)
			useGeolocation = true
		}
	}
	if useGeolocation {
		resourceRecordSet.GeoLocation = geolocation
	}

	if prop, ok := endpoint.GetProviderSpecificProperty(ProviderSpecificHealthCheckID); ok {
		resourceRecordSet.HealthCheckId = aws.String(prop.Value)
	}

	change := &route53.Change{
		Action:            aws.String(action),
		ResourceRecordSet: resourceRecordSet,
	}
	return change, nil
}

func (p *Provider) endpointsFromZoneStatus(record *v1.DNSRecord, zoneID string) ([]*v1.Endpoint, error) {
	for _, zoneStatus := range record.Status.Zones {
		if zoneStatus.DNSZone.ID == zoneID {
			return zoneStatus.Endpoints, nil
		}
	}
	return []*v1.Endpoint{}, nil
}
