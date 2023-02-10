package dns

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lithammer/shortuuid/v4"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/metadata"
	v1 "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns/aws"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
)

const (
	labelRecordID                  = "kuadrant.io/record-id"
	PATCH_ANNOTATION_PREFIX string = "MCTC_PATCH_"
)

var AlreadyAssignedErr = fmt.Errorf("managed host already assigned")

type Patch struct {
	OP    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value"`
}

type Service struct {
	controlClient client.Client
	// this is temporary setting the tenant ns in the control plane.
	// will be removed when we have auth that can map to a given ctrl plane ns
	defaultCtrlNS string

	hostResolver HostResolver
}

func NewService(controlClient client.Client, hostResolv HostResolver, defaultCtrlNS string) *Service {
	return &Service{controlClient: controlClient, defaultCtrlNS: defaultCtrlNS, hostResolver: hostResolv}
}

func (s *Service) resolveIPS(ctx context.Context, t traffic.Interface) ([]string, error) {
	activeDNSTargetIPs := []string{}
	targets, err := t.GetDNSTargets()
	if err != nil {
		return nil, err
	}
	for _, target := range targets {
		if target.TargetType == v1.TargetTypeIP {
			activeDNSTargetIPs = append(activeDNSTargetIPs, target.Value)
			continue
		}
		addr, err := s.hostResolver.LookupIPAddr(ctx, target.Value)
		if err != nil {
			return activeDNSTargetIPs, fmt.Errorf("DNSLookup failed for host %s : %s", target.Value, err)
		}
		for _, add := range addr {
			activeDNSTargetIPs = append(activeDNSTargetIPs, add.IP.String())
		}
	}
	return activeDNSTargetIPs, nil
}

func (s *Service) GetDNSRecords(ctx context.Context, traffic traffic.Interface) ([]*v1.DNSRecord, error) {
	// TODO improve this to use a label and list instead of gets
	hosts := traffic.GetHosts()
	records := []*v1.DNSRecord{}
	for _, host := range hosts {
		if host == "" {
			continue
		}
		record := &v1.DNSRecord{
			ObjectMeta: metav1.ObjectMeta{
				Name:      host,
				Namespace: s.defaultCtrlNS,
			},
		}
		if err := s.controlClient.Get(ctx, client.ObjectKeyFromObject(record), record); err != nil {
			if k8serrors.IsNotFound(err) {
				log.Log.V(10).Info("no dnsrecord found for host ", "host", record.Name)
				continue
			}
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Service) AddEndPoints(ctx context.Context, traffic traffic.Interface) error {
	ips, err := s.resolveIPS(ctx, traffic)
	if err != nil {
		return err
	}

	records, err := s.GetDNSRecords(ctx, traffic)
	if err != nil {
		return err
	}
	// for each managed host update dns. A managed host will have a DNSRecord in the control plane
	for _, r := range records {
		host := r.Name
		// record found update
		// check if endpoint already exists in the DNSRecord
		endpoints := []string{}
		for _, addr := range ips {
			endpointFound := false
			for _, endpoint := range r.Spec.Endpoints {
				if endpoint.DNSName == host && endpoint.SetIdentifier == addr {
					log.Log.V(3).Info("address ", addr, "already exists in record for host ", host)
					endpointFound = true
					continue
				}
			}
			if !endpointFound {
				endpoints = append(endpoints, addr)
			}
		}
		if len(r.Spec.Endpoints) == 0 {
			// they are all new endpoints
			endpoints = ips
		}
		for _, ep := range endpoints {
			endpoint := &v1.Endpoint{
				DNSName:       host,
				Targets:       []string{ep},
				RecordType:    "A",
				SetIdentifier: ep,
				RecordTTL:     60,
			}

			r.Spec.Endpoints = append(r.Spec.Endpoints, endpoint)
		}
		totalIPs := 0
		for _, e := range r.Spec.Endpoints {
			totalIPs += len(e.Targets)
		}
		for _, e := range r.Spec.Endpoints {
			e.SetProviderSpecific(aws.ProviderSpecificWeight, awsEndpointWeight(totalIPs))
		}

		return s.controlClient.Update(ctx, r, &client.UpdateOptions{})
	}
	return nil
}

func (s *Service) RemoveEndpoints(ctx context.Context, t traffic.Interface) error {
	records, err := s.GetDNSRecords(ctx, t)
	if err != nil {
		return err
	}
	ips, err := s.resolveIPS(ctx, t)
	if err != nil {
		return err
	}
	newEndpoints := []*v1.Endpoint{}
	for _, record := range records {
		log.Log.V(10).Info("removing ip from record ", "host ", record.Name)
		for _, endpoint := range record.Spec.Endpoints {
			for _, addr := range ips {
				if endpoint.SetIdentifier != addr {
					newEndpoints = append(newEndpoints, endpoint)
				}
			}
		}
		record.Spec.Endpoints = newEndpoints
		if len(record.Spec.Endpoints) == 0 {
			// TODO should it be deleted at this point if there are no endpoints all ingresses are gone? If not where do we want to make this decision.
			//record.Spec = v1.DNSRecordSpec{}
			if err := s.controlClient.Delete(ctx, record); err != nil {
				return err
			}
			return nil
		}
		if err := s.controlClient.Update(ctx, record, &client.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// EnsureManagedHost will ensure there is at least one managed host for the traffic object and return those host and dnsrecords
func (s *Service) EnsureManagedHost(ctx context.Context, t traffic.Interface) ([]string, []*v1.DNSRecord, error) {
	dnsRecords, err := s.GetDNSRecords(ctx, t)
	var managedHosts []string
	if err != nil {
		return managedHosts, nil, err
	}

	if len(dnsRecords) != 0 {
		for _, r := range dnsRecords {
			managedHosts = append(managedHosts, r.Name)
		}
		return managedHosts, dnsRecords, AlreadyAssignedErr
	}

	log.Log.Info("no managed host found generating one")
	hostKey := shortuuid.NewWithNamespace(t.GetNamespace() + t.GetName())
	zones := getManagedZones()
	var chosenZone zone
	var managedHost string
	for _, z := range zones {
		if z.Default {
			managedHost = strings.ToLower(fmt.Sprintf("%s.%s", hostKey, z.RootDomain))
			chosenZone = z
			break
		}
	}
	if chosenZone.ID == "" {
		return managedHosts, dnsRecords, fmt.Errorf("no zone available to use")
	}
	record, err := s.RegisterHost(ctx, managedHost, hostKey, chosenZone.DNSZone)
	if err != nil {
		log.Log.Error(err, "failed to register host ")
		return managedHosts, dnsRecords, err
	}
	managedHosts = append(managedHosts, managedHost)
	dnsRecords = append(dnsRecords, record)
	return managedHosts, dnsRecords, nil
}

func (s *Service) RegisterHost(ctx context.Context, h string, id string, zone v1.DNSZone) (*v1.DNSRecord, error) {
	dnsRecord := v1.DNSRecord{
		ObjectMeta: metav1.ObjectMeta{
			Name:      h,
			Namespace: s.defaultCtrlNS,
			Labels:    map[string]string{labelRecordID: id},
		},
	}

	err := s.controlClient.Create(ctx, &dnsRecord, &client.CreateOptions{})
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}
	//host may already be present
	if err != nil && k8serrors.IsAlreadyExists(err) {
		err = s.controlClient.Get(ctx, client.ObjectKeyFromObject(&dnsRecord), &dnsRecord)
		if err != nil {
			return nil, err
		}
	}
	return &dnsRecord, nil
}

func (s *Service) PatchTargets(ctx context.Context, targets, hosts []string, clusterID string, remove bool) error {
	//build patches to add dns targets to all matched DNSRecords
	patches := []*Patch{}
	for _, target := range targets {
		patch := &Patch{
			OP:    "add",
			Path:  "/spec/endpoints/0/targets/-",
			Value: target,
		}
		patches = append(patches, patch)
	}
	patchAnnotation, err := json.Marshal(patches)
	if err != nil {
		return fmt.Errorf("could not convert patches to string. Patches: %+v, error: %v", patches, err)
	}
	for _, host := range hosts {
		if host == "" {
			continue
		}
		dnsRecord := &v1.DNSRecord{}
		err := s.controlClient.Get(ctx, client.ObjectKey{Name: host, Namespace: s.defaultCtrlNS}, dnsRecord)
		if err != nil {
			return err
		}

		if !remove {
			metadata.AddAnnotation(dnsRecord, PATCH_ANNOTATION_PREFIX+clusterID, string(patchAnnotation))
		} else {
			metadata.RemoveAnnotation(dnsRecord, PATCH_ANNOTATION_PREFIX+clusterID)
		}

		err = s.controlClient.Update(ctx, dnsRecord)
		if err != nil {
			return err
		}
	}
	return nil
}

// this is temporary and will be replaced in the future by CRD resources
type zone struct {
	v1.DNSZone
	RootDomain string
	Default    bool
}

// this is temporary and will be replaced in the future by CRD resources
func getManagedZones() []zone {
	return []zone{{
		DNSZone:    v1.DNSZone{ID: os.Getenv("AWS_DNS_PUBLIC_ZONE_ID")},
		RootDomain: os.Getenv("ZONE_ROOT_DOMAIN"),
		Default:    true,
	}}
}

// awsEndpointWeight returns the weight Value for a single AWS record in a set of records where the traffic is split
// evenly between a number of clusters/ingresses, each splitting traffic evenly to a number of IPs (numIPs)
//
// Divides the number of IPs by a known weight allowance for a cluster/ingress, note that this means:
// * Will always return 1 after a certain number of ips is reached, 60 in the current case (maxWeight / 2)
// * Will return values that don't add up to the total maxWeight when the number of ingresses is not divisible by numIPs
//
// The aws weight value must be an integer between 0 and 255.
// https://docs.aws.amazon.com/Route53/latest/DeveloperGuide/resource-record-sets-values-weighted.html#rrsets-values-weighted-weight
func awsEndpointWeight(numIPs int) string {
	maxWeight := 120
	if numIPs > maxWeight {
		numIPs = maxWeight
	}
	return strconv.Itoa(maxWeight / numIPs)
}
