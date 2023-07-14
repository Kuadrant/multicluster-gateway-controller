/*
Copyright 2023 The MultiCluster Traffic Controller Authors.

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

package google

import (
	"context"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/linki/instrumented_http"
	"golang.org/x/oauth2/google"
	dnsv1 "google.golang.org/api/dns/v1"
	googleapi "google.golang.org/api/googleapi"
	"google.golang.org/api/option"
	"sigs.k8s.io/controller-runtime/pkg/log"
	//"google.golang.org/api/option"

	v1 "k8s.io/api/core/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
)

type action string

const (
	GoogleBatchChangeSize            = 1000
	GoogleBatchChangeInterval        = time.Second
	DryRun                           = false
	upsertAction              action = "UPSERT"
	deleteAction              action = "DELETE"
)

// Based on the external-dnsv1 google provider https://github.com/kubernetes-sigs/external-dns/blob/master/provider/google/google.go

type managedZonesCreateCallInterface interface {
	Do(opts ...googleapi.CallOption) (*dnsv1.ManagedZone, error)
}

type managedZonesGetCallInterface interface {
	Do(opts ...googleapi.CallOption) (*dnsv1.ManagedZone, error)
}

type managedZonesListCallInterface interface {
	Pages(ctx context.Context, f func(*dnsv1.ManagedZonesListResponse) error) error
}

type managedZonesServiceInterface interface {
	Create(project string, managedzone *dnsv1.ManagedZone) managedZonesCreateCallInterface
	Get(project string, managedZone string) managedZonesGetCallInterface
	List(project string) managedZonesListCallInterface
}

type resourceRecordSetsListCallInterface interface {
	Pages(ctx context.Context, f func(*dnsv1.ResourceRecordSetsListResponse) error) error
}

type resourceRecordSetsClientInterface interface {
	List(project string, managedZone string) resourceRecordSetsListCallInterface
}

type changesCreateCallInterface interface {
	Do(opts ...googleapi.CallOption) (*dnsv1.Change, error)
}

type changesServiceInterface interface {
	Create(project string, managedZone string, change *dnsv1.Change) changesCreateCallInterface
}

type resourceRecordSetsService struct {
	service *dnsv1.ResourceRecordSetsService
}

func (r resourceRecordSetsService) List(project string, managedZone string) resourceRecordSetsListCallInterface {
	return r.service.List(project, managedZone)
}

type managedZonesService struct {
	service *dnsv1.ManagedZonesService
}

func (m managedZonesService) Create(project string, managedzone *dnsv1.ManagedZone) managedZonesCreateCallInterface {
	return m.service.Create(project, managedzone)
}

func (m managedZonesService) Get(project string, managedZone string) managedZonesGetCallInterface {
	return m.service.Get(project, managedZone)
}

func (m managedZonesService) List(project string) managedZonesListCallInterface {
	return m.service.List(project)
}

type changesService struct {
	service *dnsv1.ChangesService
}

func (c changesService) Create(project string, managedZone string, change *dnsv1.Change) changesCreateCallInterface {
	return c.service.Create(project, managedZone, change)
}

type GoogleDNSProvider struct {
	logger logr.Logger
	// The Google project to work in
	project string
	// Enabled dry-run will print any modifying actions rather than execute them.
	dryRun bool
	// Max batch size to submit to Google Cloud DNS per transaction.
	batchChangeSize int
	// Interval between batch updates.
	batchChangeInterval time.Duration
	// A client for managing resource record sets
	resourceRecordSetsClient resourceRecordSetsClientInterface
	// A client for managing hosted zones
	managedZonesClient managedZonesServiceInterface
	// A client for managing change sets
	changesClient changesServiceInterface
	// The context parameter to be passed for gcloud API calls.
	ctx context.Context
}

var _ dns.Provider = &GoogleDNSProvider{}

func NewProviderFromSecret(ctx context.Context, s *v1.Secret) (*GoogleDNSProvider, error) {

	//ToDo client should be created using credentials from the secret
	gcloud, err := google.DefaultClient(ctx, dnsv1.NdevClouddnsReadwriteScope)
	if err != nil {
		return nil, err
	}

	gcloud = instrumented_http.NewClient(gcloud, &instrumented_http.Callbacks{
		PathProcessor: func(path string) string {
			parts := strings.Split(path, "/")
			return parts[len(parts)-1]
		},
	})

	dnsClient, err := dnsv1.NewService(ctx, option.WithHTTPClient(gcloud))
	if err != nil {
		return nil, err
	}

	//Todo This needs to be pulled out of the secret
	var project = "it-cloud-gcp-rd-midd-san"

	provider := &GoogleDNSProvider{
		logger:                   log.Log.WithName("google-dns").WithValues("project", project),
		project:                  project,
		dryRun:                   DryRun,
		batchChangeSize:          GoogleBatchChangeSize,
		batchChangeInterval:      GoogleBatchChangeInterval,
		resourceRecordSetsClient: resourceRecordSetsService{dnsClient.ResourceRecordSets},
		managedZonesClient:       managedZonesService{dnsClient.ManagedZones},
		changesClient:            changesService{dnsClient.Changes},
		ctx:                      ctx,
	}

	return provider, nil
}

func (g GoogleDNSProvider) Ensure(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	return g.updateRecord(record, managedZone.Status.ID, upsertAction)
}

func (g GoogleDNSProvider) Delete(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	return g.updateRecord(record, managedZone.Status.ID, deleteAction)
}

func (g GoogleDNSProvider) EnsureManagedZone(managedZone *v1alpha1.ManagedZone) (dns.ManagedZoneOutput, error) {
	var zoneID string
	if managedZone.Spec.ID != "" {
		zoneID = managedZone.Spec.ID
	} else {
		zoneID = managedZone.Status.ID
	}

	var managedZoneOutput dns.ManagedZoneOutput

	if zoneID != "" {
		//Get existing managed zone
		mz, err := g.managedZonesClient.Get(g.project, zoneID).Do()
		if err != nil {
			return managedZoneOutput, err
		}
		var nameservers []*string
		for _, ns := range mz.NameServers {
			nameservers = append(nameservers, &ns)
		}
		managedZoneOutput.ID = mz.Name
		managedZoneOutput.RecordCount = -1
		managedZoneOutput.NameServers = nameservers
		return managedZoneOutput, nil
	}
	//ToDo Create a new managed zone
	return managedZoneOutput, nil
}

func (g GoogleDNSProvider) DeleteManagedZone(managedZone *v1alpha1.ManagedZone) error {
	//TODO implement me
	return nil
}

func (g GoogleDNSProvider) HealthCheckReconciler() dns.HealthCheckReconciler {
	// This can be ignored and likely removed as part of the provider-agnostic health check work
	return &dns.FakeHealthCheckReconciler{}
}

func (g GoogleDNSProvider) ProviderSpecific() dns.ProviderSpecificLabels {
	return dns.ProviderSpecificLabels{}
}

func (g *GoogleDNSProvider) updateRecord(record *v1alpha1.DNSRecord, zoneID string, action action) error {
	var addingRecords []*dnsv1.ResourceRecordSet
	var deletingRecords []*dnsv1.ResourceRecordSet
	change := &dnsv1.Change{}

	for _, endpoint := range record.Spec.Endpoints {
		addingRecords = append(addingRecords, newRecord(endpoint))
	}

	for _, endpoint := range record.Status.Endpoints {
		deletingRecords = append(deletingRecords, newRecord(endpoint))
	}

	if action == deleteAction {
		change.Deletions = deletingRecords
	} else {
		change.Deletions = deletingRecords
		change.Additions = addingRecords
	}

	return g.submitChange(change, zoneID)
}

func (g *GoogleDNSProvider) submitChange(change *dnsv1.Change, zone string) error {
	if len(change.Additions) == 0 && len(change.Deletions) == 0 {
		g.logger.Info("All records are already up to date")
		return nil
	}

	for _, c := range g.batchChange(change, g.batchChangeSize) {
		//g.logger.V(1).Info("Change zone: %v batch #%d", zone, batch)
		//for _, del := range c.Deletions {
		//g.logger.V(1).Info("Del records: %s %s %s %d", del.Name, del.Type, del.Rrdatas, del.Ttl)
		//}
		//for _, add := range c.Additions {
		//g.logger.V(1).Info("Add records: %s %s %s %d", add.Name, add.Type, add.Rrdatas, add.Ttl)
		//}

		if g.dryRun {
			continue
		}

		if _, err := g.changesClient.Create(g.project, zone, c).Do(); err != nil {
			return err
		}
		time.Sleep(g.batchChangeInterval)
	}
	return nil
}

func (g *GoogleDNSProvider) batchChange(change *dnsv1.Change, batchSize int) []*dnsv1.Change {
	changes := []*dnsv1.Change{}

	if batchSize == 0 {
		return append(changes, change)
	}

	type dnsv1Change struct {
		additions []*dnsv1.ResourceRecordSet
		deletions []*dnsv1.ResourceRecordSet
	}

	changesByName := map[string]*dnsv1Change{}

	for _, a := range change.Additions {
		change, ok := changesByName[a.Name]
		if !ok {
			change = &dnsv1Change{}
			changesByName[a.Name] = change
		}

		change.additions = append(change.additions, a)
	}

	for _, a := range change.Deletions {
		change, ok := changesByName[a.Name]
		if !ok {
			change = &dnsv1Change{}
			changesByName[a.Name] = change
		}

		change.deletions = append(change.deletions, a)
	}

	names := make([]string, 0)
	for v := range changesByName {
		names = append(names, v)
	}
	sort.Strings(names)

	currentChange := &dnsv1.Change{}
	var totalChanges int
	for _, name := range names {
		c := changesByName[name]

		totalChangesByName := len(c.additions) + len(c.deletions)

		if totalChangesByName > batchSize {
			g.logger.V(1).Info("Total changes for %s exceeds max batch size of %d, total changes: %d", name,
				batchSize, totalChangesByName)
			continue
		}

		if totalChanges+totalChangesByName > batchSize {
			totalChanges = 0
			changes = append(changes, currentChange)
			currentChange = &dnsv1.Change{}
		}

		currentChange.Additions = append(currentChange.Additions, c.additions...)
		currentChange.Deletions = append(currentChange.Deletions, c.deletions...)

		totalChanges += totalChangesByName
	}

	if totalChanges > 0 {
		changes = append(changes, currentChange)
	}

	return changes
}

// newRecord returns a RecordSet based on the given endpoint.
func newRecord(ep *v1alpha1.Endpoint) *dnsv1.ResourceRecordSet {
	targets := make([]string, len(ep.Targets))
	copy(targets, []string(ep.Targets))
	if ep.RecordType == string(v1alpha1.CNAMERecordType) {
		targets[0] = ensureTrailingDot(targets[0])
	}
	return &dnsv1.ResourceRecordSet{
		Name:    ensureTrailingDot(ep.DNSName),
		Rrdatas: targets,
		Ttl:     int64(ep.RecordTTL),
		Type:    ep.RecordType,
	}
}

// ensureTrailingDot ensures that the hostname receives a trailing dot if it hasn't already.
func ensureTrailingDot(hostname string) string {
	if net.ParseIP(hostname) != nil {
		return hostname
	}

	return strings.TrimSuffix(hostname, ".") + "."
}
