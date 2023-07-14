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

const (
	GoogleBatchChangeSize     = 1000
	GoogleBatchChangeInterval = time.Second
	DryRun                    = false
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
	//TODO implement me
	panic("implement me")
}

func (g GoogleDNSProvider) Delete(record *v1alpha1.DNSRecord, managedZone *v1alpha1.ManagedZone) error {
	//TODO implement me
	panic("implement me")
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
	panic("implement me")
}

func (g GoogleDNSProvider) HealthCheckReconciler() dns.HealthCheckReconciler {
	//TODO implement me
	panic("implement me")
}

func (g GoogleDNSProvider) ProviderSpecific() dns.ProviderSpecificLabels {
	//TODO implement me
	panic("implement me")
}
