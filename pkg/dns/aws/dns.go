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
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/go-logr/logr"

	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

const (
	ProviderSpecificRegion                     = "aws/region"
	ProviderSpecificFailover                   = "aws/failover"
	ProviderSpecificGeolocationSubdivisionCode = "aws/geolocation-subdivision-code"
	ProviderSpecificMultiValueAnswer           = "aws/multi-value-answer"
	ProviderSpecificHealthCheckID              = "aws/health-check-id"
)

type Route53DNSProvider struct {
	client *InstrumentedRoute53
	logger logr.Logger
	// only consider hosted zones ending with this zone id
	zoneIDFilter dns.ZoneIDFilter
	// only consider hosted zones managing domains ending in this suffix
	domainFilter          dns.DomainFilter
	healthCheckReconciler dns.HealthCheckReconciler
}

var _ dns.Provider = &Route53DNSProvider{}

func NewProviderFromSecret(s *v1.Secret) (*Route53DNSProvider, error) {

	if string(s.Data["AWS_ACCESS_KEY_ID"]) == "" || string(s.Data["AWS_SECRET_ACCESS_KEY"]) == "" {
		return nil, fmt.Errorf("AWS Provider credentials is empty")
	}

	pConfig, err := dns.ConfigFromJSON(s.Data["CONFIG"])
	if err != nil {
		return nil, err
	}

	config := aws.NewConfig()
	sessionOpts := session.Options{
		Config: *config,
	}

	sessionOpts.Config.Credentials = credentials.NewStaticCredentials(string(s.Data["AWS_ACCESS_KEY_ID"]), string(s.Data["AWS_SECRET_ACCESS_KEY"]), "")
	sessionOpts.SharedConfigState = session.SharedConfigDisable
	sess, err := session.NewSessionWithOptions(sessionOpts)
	if err != nil {
		return nil, fmt.Errorf("unable to create aws session: %s", err)
	}
	if string(s.Data["REGION"]) != "" {
		sess.Config.WithRegion(string(s.Data["REGION"]))
	}

	zoneIDFilter := dns.NewZoneIDFilter(pConfig.ZoneIDFilter)
	domainFilter := dns.NewDomainFilter(pConfig.DomainFilter)

	p := &Route53DNSProvider{
		client:       &InstrumentedRoute53{route53.New(sess, config)},
		logger:       log.Log.WithName("aws-route53").WithValues("region", config.Region),
		zoneIDFilter: zoneIDFilter,
		domainFilter: domainFilter,
	}

	if err := validateServiceEndpoints(p); err != nil {
		return nil, fmt.Errorf("failed to validate AWS provider service endpoints: %v", err)
	}

	return p, nil
}

type action string

const (
	upsertAction action = "UPSERT"
	deleteAction action = "DELETE"
)

func (p *Route53DNSProvider) Ensure(record *v1alpha2.DNSRecord) error {
	return p.change(record, upsertAction)
}

func (p *Route53DNSProvider) Delete(record *v1alpha2.DNSRecord) error {
	return p.change(record, deleteAction)
}

func (p *Route53DNSProvider) ListZones() (dns.ZoneList, error) {
	var zoneList dns.ZoneList
	zones, err := p.zones()
	if err != nil {
		return zoneList, err
	}
	for _, zone := range zones {
		dnsName := removeTrailingDot(*zone.Name)
		zoneID := removeHostedZoneIDPrefix(*zone.Id)
		zoneList.Items = append(zoneList.Items, &dns.Zone{
			ID:      &zoneID,
			DNSName: &dnsName,
		})
	}
	return zoneList, nil
}

func (p *Route53DNSProvider) EnsureManagedZone(zone *v1alpha2.ManagedZone) (dns.ManagedZoneOutput, error) {
	var zoneID string
	if zone.Spec.ID != nil {
		zoneID = *zone.Spec.ID
	} else {
		zoneID = zone.Status.ID
	}

	var managedZoneOutput dns.ManagedZoneOutput

	if zoneID != "" {
		getResp, err := p.client.GetHostedZone(&route53.GetHostedZoneInput{
			Id: &zoneID,
		})
		if err != nil {
			log.Log.Error(err, "failed to get hosted zone")
			return managedZoneOutput, err
		}

		//Only update if we created the managed zone and description is set
		if zone.Spec.ID != nil && zone.Spec.Description != nil {
			_, err = p.client.UpdateHostedZoneComment(&route53.UpdateHostedZoneCommentInput{
				Comment: zone.Spec.Description,
				Id:      &zoneID,
			})
			if err != nil {
				log.Log.Error(err, "failed to update hosted zone comment")
			}
		}

		managedZoneOutput.ID = removeHostedZoneIDPrefix(*getResp.HostedZone.Id)
		managedZoneOutput.RecordCount = *getResp.HostedZone.ResourceRecordSetCount
		managedZoneOutput.NameServers = getResp.DelegationSet.NameServers

		return managedZoneOutput, nil
	}

	//ToDo callerRef must be unique, but this can cause duplicates if the status can't be written back during a
	//reconciliation that successfully created a new hosted zone i.e. the object has been modified; please apply your
	//changes to the latest version and try again
	callerRef := time.Now().Format("20060102150405")
	// Create the hosted zone
	createResp, err := p.client.CreateHostedZone(&route53.CreateHostedZoneInput{
		CallerReference: &callerRef,
		Name:            &zone.Spec.DomainName,
		HostedZoneConfig: &route53.HostedZoneConfig{
			Comment:     zone.Spec.Description,
			PrivateZone: aws.Bool(false),
		},
	})
	if err != nil {
		log.Log.Error(err, "failed to create hosted zone")
		return managedZoneOutput, err
	}
	managedZoneOutput.ID = *createResp.HostedZone.Id
	managedZoneOutput.RecordCount = *createResp.HostedZone.ResourceRecordSetCount
	managedZoneOutput.NameServers = createResp.DelegationSet.NameServers
	return managedZoneOutput, nil
}

func (p *Route53DNSProvider) DeleteManagedZone(zone *v1alpha2.ManagedZone) error {
	_, err := p.client.DeleteHostedZone(&route53.DeleteHostedZoneInput{
		Id: &zone.Status.ID,
	})
	if err != nil {
		log.Log.Error(err, "failed to delete hosted zone")
		return err
	}
	return nil
}

func (p *Route53DNSProvider) HealthCheckReconciler() dns.HealthCheckReconciler {
	if p.healthCheckReconciler == nil {
		p.healthCheckReconciler = dns.NewCachedHealthCheckReconciler(
			p,
			NewRoute53HealthCheckReconciler(p.client.route53),
		)
	}

	return p.healthCheckReconciler
}

func (*Route53DNSProvider) ProviderSpecific() dns.ProviderSpecificLabels {
	return dns.ProviderSpecificLabels{
		Weight:        dns.ProviderSpecificWeight,
		HealthCheckID: ProviderSpecificHealthCheckID,
	}
}

// Zones returns the list of hosted zones.
func (p *Route53DNSProvider) zones() (map[string]*route53.HostedZone, error) {
	zones := make(map[string]*route53.HostedZone)

	f := func(resp *route53.ListHostedZonesOutput, lastPage bool) (shouldContinue bool) {
		for _, zone := range resp.HostedZones {
			if !p.domainFilter.Match(aws.StringValue(zone.Name)) && !p.zoneIDFilter.Match(aws.StringValue(zone.Id)) {
				continue
			}
			zones[aws.StringValue(zone.Id)] = zone
		}
		return true
	}

	err := p.client.route53.ListHostedZonesPages(&route53.ListHostedZonesInput{}, f)
	if err != nil {
		return nil, fmt.Errorf("failed to list hosted zones: %w", err)
	}

	for _, zone := range zones {
		log.Log.V(1).Info("Considering zone", "zone.Id", aws.StringValue(zone.Id), "zone.Name", aws.StringValue(zone.Name))
	}

	return zones, nil
}

func (p *Route53DNSProvider) change(record *v1alpha2.DNSRecord, action action) error {
	// Configure records.
	if len(record.Spec.Endpoints) == 0 {
		return nil
	}
	err := p.updateRecord(record, string(action))
	if err != nil {
		return fmt.Errorf("failed to update record in route53 hosted zone %s: %w", *record.Spec.ZoneID, err)
	}
	switch action {
	case upsertAction:
		p.logger.Info("Upserted DNS record", "record", record.Spec, "hostedZoneID", record.Spec.ZoneID)
	case deleteAction:
		p.logger.Info("Deleted DNS record", "record", record.Spec, "hostedZoneID", record.Spec.ZoneID)
	}
	return nil
}

func (p *Route53DNSProvider) updateRecord(record *v1alpha2.DNSRecord, action string) error {

	if len(record.Spec.Endpoints) == 0 {
		return fmt.Errorf("no endpoints")
	}

	input := route53.ChangeResourceRecordSetsInput{HostedZoneId: aws.String(*record.Spec.ZoneID)}

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
		lastPublishedEndpoints := record.Status.Endpoints
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
	resp, err := p.client.ChangeResourceRecordSets(&input)
	if err != nil {
		return fmt.Errorf("couldn't update DNS record %s in zone %s: %v", record.Name, *record.Spec.ZoneID, err)
	}
	p.logger.Info("Updated DNS record", "record", record, "zone", *record.Spec.ZoneID, "response", resp)
	return nil
}

func (p *Route53DNSProvider) changeForEndpoint(endpoint *v1alpha2.Endpoint, action string) (*route53.Change, error) {
	if endpoint.RecordType != string(v1alpha2.ARecordType) && endpoint.RecordType != string(v1alpha2.CNAMERecordType) && endpoint.RecordType != string(v1alpha2.NSRecordType) {
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
	if prop, ok := endpoint.GetProviderSpecificProperty(dns.ProviderSpecificWeight); ok {
		weight, err := strconv.ParseInt(prop.Value, 10, 64)
		if err != nil {
			p.logger.Error(err, "Failed parsing value, using weight of 0", "weight", dns.ProviderSpecificWeight, "value", prop.Value)
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

	if prop, ok := endpoint.GetProviderSpecificProperty(dns.ProviderSpecificGeoCode); ok {
		if dns.IsISO3166Alpha2Code(prop.Value) || dns.GeoCode(prop.Value).IsWildcard() {
			geolocation.CountryCode = aws.String(prop.Value)
		} else {
			geolocation.ContinentCode = aws.String(prop.Value)
		}
		useGeolocation = true
	}

	if geolocation.ContinentCode == nil {
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

// validateServiceEndpoints validates that provider clients can communicate with
// associated API endpoints by having each client make a list/describe/get call.
func validateServiceEndpoints(provider *Route53DNSProvider) error {
	var errs []error
	zoneInput := route53.ListHostedZonesInput{MaxItems: aws.String("1")}
	if _, err := provider.client.ListHostedZones(&zoneInput); err != nil {
		errs = append(errs, fmt.Errorf("failed to list route53 hosted zones: %v", err))
	}
	return kerrors.NewAggregate(errs)
}

// removeTrailingDot ensures that the hostname receives a trailing dot if it hasn't already.
func removeTrailingDot(hostname string) string {
	if net.ParseIP(hostname) != nil {
		return hostname
	}

	return strings.TrimSuffix(hostname, ".")
}

func removeHostedZoneIDPrefix(id string) string {
	return strings.TrimPrefix(id, "/hostedzone/")
}
