//go:build unit

package dnspolicy

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func testScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("falied to add work scheme %s ", err)
	}
	if err := gatewayv1beta1.AddToScheme(scheme); err != nil {
		t.Fatalf("falied to add work scheme %s ", err)
	}
	return scheme
}

func TestSetProviderSpecific(t *testing.T) {
	endpoint := &v1alpha1.Endpoint{
		ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
			{Name: "weight", Value: "120"},
		},
	}

	// Test updating an existing property
	endpoint.SetProviderSpecific("weight", "60")
	for _, property := range endpoint.ProviderSpecific {
		if property.Name == "weight" && property.Value != "60" {
			t.Errorf("Existing property was not updated. Got %s, expected 60", property.Value)
		}
	}
}

func Test_dnsHelper_createDNSRecord(t *testing.T) {
	type args struct {
		gateway     *gatewayv1beta1.Gateway
		dnsPolicy   *v1alpha1.DNSPolicy
		subDomain   string
		managedZone *v1alpha1.ManagedZone
	}
	tests := []struct {
		name       string
		args       args
		recordList *v1alpha1.DNSRecordList
		wantRecord *v1alpha1.DNSRecord
		wantErr    bool
	}{
		{
			name: "DNS record gets created",
			args: args{
				gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstgateway",
						Namespace: "test",
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstpolicy",
						Namespace: "test",
					},
				},
				subDomain: "sub",
				managedZone: &v1alpha1.ManagedZone{
					ObjectMeta: v1.ObjectMeta{
						Name:      "mz",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "domain.com",
					},
				},
			},
			recordList: &v1alpha1.DNSRecordList{},
			wantRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "sub.domain.com",
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						"gateway-namespace":               "test",
						"gateway":                         "tstgateway",
					},
					OwnerReferences: []v1.OwnerReference{
						{
							APIVersion:         "kuadrant.io/v1alpha1",
							Kind:               "ManagedZone",
							Name:               "mz",
							Controller:         testutil.Pointer(true),
							BlockOwnerDeletion: testutil.Pointer(true),
						},
					},
					ResourceVersion: "1",
				},
				Spec: v1alpha1.DNSRecordSpec{
					ManagedZoneRef: &v1alpha1.ManagedZoneReference{
						Name: "mz",
					},
				},
			},
		},
		{
			name: "DNS record already exists",
			args: args{
				gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstgateway",
						Namespace: "test",
					},
				},
				dnsPolicy: &v1alpha1.DNSPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstpolicy",
						Namespace: "test",
					},
				},
				subDomain: "sub",
				managedZone: &v1alpha1.ManagedZone{
					ObjectMeta: v1.ObjectMeta{
						Name:      "mz",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "domain.com",
					},
				},
			},
			recordList: &v1alpha1.DNSRecordList{
				Items: []v1alpha1.DNSRecord{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      "sub.domain.com",
							Namespace: "test",
						},
					},
				},
			},
			wantRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:            "sub.domain.com",
					Namespace:       "test",
					ResourceVersion: "999",
				},
				TypeMeta: v1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "kuadrant.io/v1alpha1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithLists(tt.recordList).Build()
			s := dnsHelper{Client: f}

			gotRecord, err := s.createDNSRecord(context.TODO(), tt.args.gateway, tt.args.dnsPolicy, tt.args.subDomain, tt.args.managedZone)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDNSRecord() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !equality.Semantic.DeepEqual(gotRecord, tt.wantRecord) {
				t.Errorf("CreateDNSRecord() gotRecord = \n%v, want \n%v", gotRecord, tt.wantRecord)
			}
		})
	}
}

func Test_dnsHelper_findMatchingManagedZone(t *testing.T) {
	cases := []struct {
		Name   string
		Host   string
		Zones  []v1alpha1.ManagedZone
		Assert func(t *testing.T, zone *v1alpha1.ManagedZone, subdomain string, err error)
	}{
		{
			Name: "finds the matching managed zone",
			Host: "sub.domain.test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "example.com",
					},
				},
			},
			Assert: assertSub("example.com", "sub.domain.test", ""),
		},
		{
			Name: "finds the most exactly matching managed zone",
			Host: "sub.domain.test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "example.com",
					},
				},
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test.example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "test.example.com",
					},
				},
			},
			Assert: assertSub("test.example.com", "sub.domain", ""),
		},
		{
			Name: "returns a single subdomain",
			Host: "sub.test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test.example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "test.example.com",
					},
				},
			},
			Assert: assertSub("test.example.com", "sub", ""),
		},
		{
			Name: "returns an error when nothing matches",
			Host: "sub.test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testing.example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "testing.example.com",
					},
				},
			},
			Assert: assertSub("", "", "no valid zone found"),
		},
		{
			Name: "handles TLD with a dot",
			Host: "sub.domain.test.example.co.uk",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "example.co.uk",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "example.co.uk",
					},
				},
			},
			Assert: assertSub("example.co.uk", "sub.domain.test", ""),
		},
		{
			Name: "TLD with a . will not match against a managedzone of the TLD",
			Host: "sub.domain.test.example.co.uk",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "co.uk",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "co.uk",
					},
				},
			},
			Assert: assertSub("", "", "no valid zone found"),
		},
		{
			Name:  "no managed zones for host give error",
			Host:  "sub.domain.test.example.co.uk",
			Zones: []v1alpha1.ManagedZone{},
			Assert: func(t *testing.T, zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if err == nil {
					t.Fatalf("expected error, got %v", err)
				}
			},
		},
		{
			Name: "should not match when host and zone domain name are identical",
			Host: "test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: v1.ObjectMeta{
						Name:      "test.example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "test.example.com",
					},
				},
			},
			Assert: assertSub("", "", "no valid zone found"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			mx, subDomain, err := findMatchingManagedZone(tc.Host, tc.Host, tc.Zones)
			tc.Assert(t, mx, subDomain, err)
		})
	}
}

func Test_dnsHelper_setEndpoints(t *testing.T) {
	listener := func(hostName string) *gatewayv1beta1.Listener {
		host := gatewayv1beta1.Hostname(hostName)
		return &gatewayv1beta1.Listener{
			Name:     "test",
			Hostname: &host,
		}
	}

	tests := []struct {
		name      string
		mcgTarget *dns.MultiClusterGatewayTarget
		listener  *gatewayv1beta1.Listener
		dnsRecord *v1alpha1.DNSRecord
		wantSpec  *v1alpha1.DNSRecordSpec
		wantErr   bool
	}{
		{
			name:     "sets weighted endpoints",
			listener: listener("test.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testgw",
						Namespace: "testns",
					},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &dns.ClusterGateway{
							ClusterName: "test-cluster-1",
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "1.1.1.1",
								},
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "2.2.2.2",
								},
							},
						},
						Geo:    testutil.Pointer(dns.GeoCode("default")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &dns.ClusterGateway{
							ClusterName: "test-cluster-2",
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.HostnameAddressType),
									Value: "mylb.example.com",
								},
							},
						},
						Geo:    testutil.Pointer(dns.GeoCode("default")),
						Weight: testutil.Pointer(120),
					},
				},
			},
			dnsRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name: "test.example.com",
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-0ecjaw.test.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "default.lb-0ecjaw.test.example.com",
						Targets:       []string{"20qri0.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-0ecjaw.test.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "default.lb-0ecjaw.test.example.com",
						Targets:       []string{"mylb.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "mylb.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "lb-0ecjaw.test.example.com",
						Targets:       []string{"default.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName:    "test.example.com",
						Targets:    []string{"lb-0ecjaw.test.example.com"},
						RecordType: "CNAME",
						RecordTTL:  dns.DefaultCnameTTL,
					},
				},
			},
		},
		{
			name:     "sets geo weighted endpoints",
			listener: listener("test.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testgw",
						Namespace: "testns",
					},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &dns.ClusterGateway{
							ClusterName: "test-cluster-1",
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "1.1.1.1",
								},
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "2.2.2.2",
								},
							},
						},
						Geo:    testutil.Pointer(dns.GeoCode("NA")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &dns.ClusterGateway{
							ClusterName: "test-cluster-2",
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.HostnameAddressType),
									Value: "mylb.example.com",
								},
							},
						},
						Geo:    testutil.Pointer(dns.GeoCode("IE")),
						Weight: testutil.Pointer(120),
					},
				},
				LoadBalancing: &v1alpha1.LoadBalancingSpec{
					Geo: &v1alpha1.LoadBalancingGeo{
						DefaultGeo: "NA",
					},
				},
			},
			dnsRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name: "test.example.com",
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-0ecjaw.test.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "na.lb-0ecjaw.test.example.com",
						Targets:       []string{"20qri0.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-0ecjaw.test.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "ie.lb-0ecjaw.test.example.com",
						Targets:       []string{"mylb.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "mylb.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "lb-0ecjaw.test.example.com",
						Targets:       []string{"na.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName:       "lb-0ecjaw.test.example.com",
						Targets:       []string{"na.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "NA",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "NA",
							},
						},
					},
					{
						DNSName:       "lb-0ecjaw.test.example.com",
						Targets:       []string{"ie.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "IE",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "IE",
							},
						},
					},
					{
						DNSName:    "test.example.com",
						Targets:    []string{"lb-0ecjaw.test.example.com"},
						RecordType: "CNAME",
						RecordTTL:  dns.DefaultCnameTTL,
					},
				},
			},
		},
		{
			name:     "sets no endpoints when no target addresses",
			listener: listener("test.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testgw",
						Namespace: "testns",
					},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &dns.ClusterGateway{
							ClusterName:      "test-cluster-1",
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{},
						},
						Geo:    testutil.Pointer(dns.GeoCode("NA")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &dns.ClusterGateway{
							ClusterName:      "test-cluster-2",
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{},
						},
						Geo:    testutil.Pointer(dns.GeoCode("IE")),
						Weight: testutil.Pointer(120),
					},
				},
				LoadBalancing: &v1alpha1.LoadBalancingSpec{
					Geo: &v1alpha1.LoadBalancingGeo{
						DefaultGeo: "NA",
					},
				},
			},
			dnsRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name: "test.example.com",
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(tt.dnsRecord).Build()
			s := dnsHelper{Client: f}
			if err := s.setEndpoints(context.TODO(), tt.mcgTarget, tt.dnsRecord, tt.listener); (err != nil) != tt.wantErr {
				t.Errorf("SetEndpoints() error = %v, wantErr %v", err, tt.wantErr)
			}

			gotRecord := &v1alpha1.DNSRecord{}
			if err := f.Get(context.TODO(), client.ObjectKeyFromObject(tt.dnsRecord), gotRecord); err != nil {
				t.Errorf("error gettinging updated DNSrecord")
			} else {

				sort.Slice(gotRecord.Spec.Endpoints, func(i, j int) bool {
					id1 := gotRecord.Spec.Endpoints[i].DNSName + gotRecord.Spec.Endpoints[i].SetIdentifier
					id2 := gotRecord.Spec.Endpoints[j].DNSName + gotRecord.Spec.Endpoints[j].SetIdentifier
					return id1 < id2
				})

				sort.Slice(tt.wantSpec.Endpoints, func(i, j int) bool {
					id1 := tt.wantSpec.Endpoints[i].DNSName + tt.wantSpec.Endpoints[i].SetIdentifier
					id2 := tt.wantSpec.Endpoints[j].DNSName + tt.wantSpec.Endpoints[j].SetIdentifier
					return id1 < id2
				})

				if !equality.Semantic.DeepEqual(gotRecord.Spec.Endpoints, tt.wantSpec.Endpoints) {
					t.Errorf("SetEndpoints() updated DNSRecord spec: \n%v, want spec: \n%v", gotRecord.Spec, *tt.wantSpec)
				}
			}

		})
	}
}

func Test_dnsHelper_getDNSRecords(t *testing.T) {
	cases := []struct {
		Name      string
		MZ        *v1alpha1.ManagedZone
		SubDomain string
		Assert    func(t *testing.T, err error)
		DNSRecord *v1alpha1.DNSRecord
		Gateway   *gatewayv1beta1.Gateway
		DNSPolicy *v1alpha1.DNSPolicy
	}{
		{
			Name: "test get dns record returns record",
			MZ: &v1alpha1.ManagedZone{

				ObjectMeta: v1.ObjectMeta{
					Name:      "b.c.com",
					Namespace: "test",
				},
				Spec: v1alpha1.ManagedZoneSpec{
					DomainName: "b.c.com",
				},
			},
			SubDomain: "a",
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "a.b.c.com",
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						"gateway-namespace":               "test",
						"gateway":                         "tstgateway",
					},
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstgateway",
					Namespace: "test",
				},
			},
			DNSPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstpolicy",
					Namespace: "test",
				},
			},
			Assert: testutil.AssertError(""),
		},
		{
			Name: "test get dns error when not found",
			MZ: &v1alpha1.ManagedZone{
				ObjectMeta: v1.ObjectMeta{
					Name:      "b.c.com",
					Namespace: "test",
				},
				Spec: v1alpha1.ManagedZoneSpec{
					DomainName: "b.c.com",
				},
			},
			SubDomain: "a",
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "other.com",
					Namespace: "test",
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstgateway",
					Namespace: "test",
				},
			},
			DNSPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstpolicy",
					Namespace: "test",
				},
			},
			Assert: testutil.AssertError("not found"),
		},
		{
			Name: "test get dns error when referencing different Gateway",
			MZ: &v1alpha1.ManagedZone{
				ObjectMeta: v1.ObjectMeta{
					Name:      "b.c.com",
					Namespace: "test",
				},
				Spec: v1alpha1.ManagedZoneSpec{
					DomainName: "b.c.com",
				},
			},
			SubDomain: "a",
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "a.b.c.com",
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "different-tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						"gateway-namespace":               "test",
						"gateway":                         "different-gateway",
					},
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstgateway",
					Namespace: "test",
				},
			},
			DNSPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstpolicy",
					Namespace: "test",
				},
			},
			Assert: testutil.AssertError("host already in use"),
		},
		{
			Name: "test get dns error when not owned by Gateway",
			MZ: &v1alpha1.ManagedZone{
				ObjectMeta: v1.ObjectMeta{
					Name:      "b.c.com",
					Namespace: "test",
				},
				Spec: v1alpha1.ManagedZoneSpec{
					DomainName: "b.c.com",
				},
			},
			SubDomain: "a",
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "other.com",
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "different-tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						"gateway-namespace":               "test",
						"gateway":                         "different-gateway",
					},
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstgateway",
					Namespace: "test",
				},
			},
			DNSPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstpolicy",
					Namespace: "test",
				},
			},
			Assert: testutil.AssertError("not found"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testutil.GetValidTestScheme()).WithObjects(tc.DNSRecord).Build()
			s := &dnsHelper{Client: f}
			_, err := s.getDNSRecord(context.TODO(), tc.Gateway, tc.DNSPolicy, tc.SubDomain, tc.MZ)
			tc.Assert(t, err)
		})
	}

}

func Test_dnsHelper_getManagedZoneForHost(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		gateway       *gatewayv1beta1.Gateway
		mz            *v1alpha1.ManagedZoneList
		scheme        *runtime.Scheme
		wantSubdomain string
		wantErr       bool
	}{
		{
			name: "found MZ",
			host: "test.example.com",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test",
				},
			},
			mz: &v1alpha1.ManagedZoneList{
				Items: []v1alpha1.ManagedZone{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      "example.com",
							Namespace: "test",
						},
						Spec: v1alpha1.ManagedZoneSpec{
							DomainName: "example.com",
						},
					},
				},
			},
			scheme:        testScheme(t),
			wantSubdomain: "test",
			wantErr:       false,
		},
		{
			name: "unable to list MZ",
			host: "example.com",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Namespace: "test",
				},
			},
			mz:      &v1alpha1.ManagedZoneList{},
			scheme:  runtime.NewScheme(),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := traffic.NewGateway(tt.gateway)
			f := fake.NewClientBuilder().WithScheme(tt.scheme).WithLists(tt.mz).Build()
			s := &dnsHelper{Client: f}

			gotMZ, gotSubdomain, err := s.getManagedZoneForHost(context.TODO(), tt.host, gw)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetManagedZoneForHost() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(gotMZ.ObjectMeta, tt.mz.Items[0].ObjectMeta) {
				t.Errorf("GetManagedZoneForHost() gotMZ = %v, want %v", gotMZ, tt.mz.Items[0])
			}
			if gotSubdomain != tt.wantSubdomain {
				t.Errorf("GetManagedZoneForHost() gotSubdomain = %v, want %v", gotSubdomain, tt.wantSubdomain)
			}
		})
	}
}

func Test_dnsHelper_getManagedHosts(t *testing.T) {
	tests := []struct {
		name      string
		gateway   *gatewayv1beta1.Gateway
		dnsPolicy *v1alpha1.DNSPolicy
		initLists []client.ObjectList
		want      []v1alpha1.ManagedHost
		wantErr   bool
	}{
		{
			name: "got managed hosts",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstgateway",
					Namespace: "test",
				},
				Spec: gatewayv1beta1.GatewaySpec{
					Listeners: []gatewayv1beta1.Listener{
						{
							Hostname: testutil.Pointer(gatewayv1beta1.Hostname("sub.domain.com")),
						},
					},
				},
			},
			dnsPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstpolicy",
					Namespace: "test",
				},
			},
			initLists: []client.ObjectList{
				&v1alpha1.ManagedZoneList{
					Items: []v1alpha1.ManagedZone{
						{
							ObjectMeta: v1.ObjectMeta{
								Namespace: "test",
							},
							Spec: v1alpha1.ManagedZoneSpec{
								DomainName: "domain.com",
							},
						},
					},
				},
				&v1alpha1.DNSRecordList{
					Items: []v1alpha1.DNSRecord{
						{
							ObjectMeta: v1.ObjectMeta{
								Name:      "sub.domain.com",
								Namespace: "test",
								Labels: map[string]string{
									"kuadrant.io/dnspolicy":           "tstpolicy",
									"kuadrant.io/dnspolicy-namespace": "test",
									"gateway-namespace":               "test",
									"gateway":                         "gateway",
								},
							},
						},
					},
				},
			},
			want: []v1alpha1.ManagedHost{
				{
					Subdomain: "sub",
					Host:      "sub.domain.com",
					ManagedZone: &v1alpha1.ManagedZone{
						ObjectMeta: v1.ObjectMeta{
							Namespace:       "test",
							ResourceVersion: "999",
						},
						Spec: v1alpha1.ManagedZoneSpec{
							DomainName: "domain.com",
						},
					},
					DnsRecord: &v1alpha1.DNSRecord{
						ObjectMeta: v1.ObjectMeta{
							Name:            "sub.domain.com",
							Namespace:       "test",
							ResourceVersion: "999",
							Labels:          map[string]string{},
						},
						TypeMeta: v1.TypeMeta{
							Kind:       "DNSRecord",
							APIVersion: "kuadrant.io/v1alpha1",
						},
					},
				},
			},
		},
		{
			name: "No hosts retrieved for CNAME or externally managed host",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstgateway",
					Namespace: "test",
				},
				Spec: gatewayv1beta1.GatewaySpec{
					Listeners: []gatewayv1beta1.Listener{
						{
							Hostname: testutil.Pointer(gatewayv1beta1.Hostname("sub.domain.com")),
						},
					},
				},
			},
			dnsPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstpolicy",
					Namespace: "test",
				},
			},
			initLists: []client.ObjectList{},
			want:      []v1alpha1.ManagedHost{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithLists(tt.initLists...).Build()
			s := &dnsHelper{Client: f}

			got, err := s.getManagedHosts(context.TODO(), tt.gateway, tt.dnsPolicy)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetManagedHosts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(got) != len(tt.want) && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetManagedHosts() got = \n%v, want \n%v", got, tt.want)
			}
		})
	}
}

func assertSub(domain string, subdomain string, err string) func(t *testing.T, expectedzone *v1alpha1.ManagedZone, expectedsubdomain string, expectedErr error) {
	return func(t *testing.T, expectedzone *v1alpha1.ManagedZone, expectedsubdomain string, expectedErr error) {
		if (err == "") != (expectedErr == nil) {
			t.Errorf("expected error '%s' but got '%s'", err, expectedErr)
		}
		if expectedErr != nil && !strings.Contains(expectedErr.Error(), err) {
			t.Errorf("expected error to be '%s' but got '%s'", err, expectedErr)
		}
		if subdomain != expectedsubdomain {
			t.Fatalf("expected subdomain '%v', got '%v'", subdomain, expectedsubdomain)
		}
		if expectedzone != nil && domain != expectedzone.Spec.DomainName {
			t.Fatalf("expected zone with domain name '%v', got '%v'", domain, expectedzone)
		}
		if expectedzone == nil && domain != "" {
			t.Fatalf("expected zone to be '%v', got '%v'", domain, expectedzone)
		}
	}
}
