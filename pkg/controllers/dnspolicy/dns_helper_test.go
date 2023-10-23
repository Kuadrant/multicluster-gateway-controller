//go:build unit

package dnspolicy

import (
	"context"
	"sort"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha2"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/utils"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func testScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("falied to add work scheme %s ", err)
	}
	if err := v1alpha2.AddToScheme(scheme); err != nil {
		t.Fatalf("falied to add work scheme %s ", err)
	}
	if err := gatewayapiv1.AddToScheme(scheme); err != nil {
		t.Fatalf("falied to add work scheme %s ", err)
	}
	return scheme
}

func getTestListener(hostName string) gatewayapiv1.Listener {
	host := gatewayapiv1.Hostname(hostName)
	return gatewayapiv1.Listener{
		Name:     "test",
		Hostname: &host,
	}
}

func TestSetProviderSpecific(t *testing.T) {
	endpoint := &v1alpha2.Endpoint{
		ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
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

func Test_dnsHelper_createDNSRecordForListener(t *testing.T) {
	var (
		testGatewayName  = "tstgateway"
		testListenerName = "test"
	)
	type args struct {
		gateway   *gatewayapiv1.Gateway
		dnsPolicy *v1alpha2.DNSPolicy
		zone      *dns.Zone
		listener  gatewayapiv1.Listener
	}
	testCases := []struct {
		name       string
		args       args
		recordList *v1alpha2.DNSRecordList
		wantRecord *v1alpha2.DNSRecord
		wantErr    bool
	}{
		{
			name: "DNS record gets created",
			args: args{
				gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      testGatewayName,
						Namespace: "test",
					},
				},
				listener: getTestListener("test.domain.com"),
				dnsPolicy: &v1alpha2.DNSPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstpolicy",
						Namespace: "test",
					},
				},
				zone: &dns.Zone{
					ID:      testutil.Pointer("mz"),
					DNSName: testutil.Pointer("domain.com"),
				},
			},
			recordList: &v1alpha2.DNSRecordList{},
			wantRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      dnsRecordName(testGatewayName, testListenerName),
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						LabelGatewayNSRef:                 "test",
						LabelGatewayReference:             "tstgateway",
						LabelListenerReference:            testListenerName,
					},
					ResourceVersion: "1",
				},
				Spec: v1alpha2.DNSRecordSpec{
					ZoneID: testutil.Pointer("mz"),
				},
			},
		},
		{
			name: "DNS record already exists",
			args: args{
				gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      testGatewayName,
						Namespace: "test",
					},
				},
				listener: getTestListener("test.domain.com"),
				dnsPolicy: &v1alpha2.DNSPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstpolicy",
						Namespace: "test",
					},
				},
				zone: &dns.Zone{
					ID:      testutil.Pointer("mz"),
					DNSName: testutil.Pointer("domain.com"),
				},
			},
			recordList: &v1alpha2.DNSRecordList{
				Items: []v1alpha2.DNSRecord{
					{
						ObjectMeta: v1.ObjectMeta{
							Name:      dnsRecordName(testGatewayName, testListenerName),
							Namespace: "test",
						},
					},
				},
			},
			wantRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:            dnsRecordName(testGatewayName, testListenerName),
					Namespace:       "test",
					ResourceVersion: "999",
				},
				TypeMeta: v1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "kuadrant.io/v1alpha2",
				},
			},
		},
		{
			name: "DNS record for wildcard listener",
			args: args{
				gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      testGatewayName,
						Namespace: "test",
					},
				},
				listener: getTestListener("*.domain.com"),
				dnsPolicy: &v1alpha2.DNSPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstpolicy",
						Namespace: "test",
					},
				},
				zone: &dns.Zone{
					ID:      testutil.Pointer("mz"),
					DNSName: testutil.Pointer("domain.com"),
				},
			},
			recordList: &v1alpha2.DNSRecordList{
				Items: []v1alpha2.DNSRecord{},
			},
			wantRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      dnsRecordName(testGatewayName, testListenerName),
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						LabelGatewayNSRef:                 "test",
						LabelGatewayReference:             "tstgateway",
						LabelListenerReference:            testListenerName,
					},
					ResourceVersion: "1",
				},
				Spec: v1alpha2.DNSRecordSpec{
					ZoneID: testutil.Pointer("mz"),
				},
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithLists(testCase.recordList).Build()
			s := dnsHelper{Client: f}

			gotRecord, err := s.createDNSRecordForListener(context.TODO(), testCase.args.gateway, testCase.args.dnsPolicy, testCase.args.listener, testCase.args.zone)
			if (err != nil) != testCase.wantErr {
				t.Errorf("CreateDNSRecord() error = %v, wantErr %v", err, testCase.wantErr)
				return
			}
			if !equality.Semantic.DeepEqual(gotRecord, testCase.wantRecord) {
				t.Errorf("CreateDNSRecord() gotRecord = \n%v, want \n%v", gotRecord, testCase.wantRecord)
			}
		})
	}
}

func Test_dnsHelper_findMatchingZone(t *testing.T) {
	testCases := []struct {
		name   string
		Host   string
		Zones  dns.ZoneList
		Assert func(t *testing.T, zone *dns.Zone, subdomain string, err error)
	}{
		{
			name: "finds the matching managed zone",
			Host: "sub.domain.test.example.com",
			Zones: dns.ZoneList{
				Items: []*dns.Zone{
					{
						ID:      testutil.Pointer("example.com"),
						DNSName: testutil.Pointer("example.com"),
					},
				},
			},
			Assert: assertSub("example.com", "sub.domain.test", ""),
		},
		{
			name: "finds the most exactly matching managed zone",
			Host: "sub.domain.test.example.com",
			Zones: dns.ZoneList{
				Items: []*dns.Zone{
					{
						ID:      testutil.Pointer("example.com"),
						DNSName: testutil.Pointer("example.com"),
					},
					{
						ID:      testutil.Pointer("test.example.com"),
						DNSName: testutil.Pointer("test.example.com"),
					},
				},
			},
			Assert: assertSub("test.example.com", "sub.domain", ""),
		},
		{
			name: "returns a single subdomain",
			Host: "sub.test.example.com",
			Zones: dns.ZoneList{
				Items: []*dns.Zone{
					{
						ID:      testutil.Pointer("test.example.com"),
						DNSName: testutil.Pointer("test.example.com"),
					},
				},
			},
			Assert: assertSub("test.example.com", "sub", ""),
		},
		{
			name: "returns an error when nothing matches",
			Host: "sub.test.example.com",
			Zones: dns.ZoneList{
				Items: []*dns.Zone{
					{
						ID:      testutil.Pointer("testing.example.com"),
						DNSName: testutil.Pointer("testing.example.com"),
					},
				},
			},
			Assert: assertSub("", "", "no valid zone found"),
		},
		{
			name: "handles TLD with a dot",
			Host: "sub.domain.test.example.co.uk",
			Zones: dns.ZoneList{
				Items: []*dns.Zone{
					{
						ID:      testutil.Pointer("example.co.uk"),
						DNSName: testutil.Pointer("example.co.uk"),
					},
				},
			},
			Assert: assertSub("example.co.uk", "sub.domain.test", ""),
		},
		{
			name: "TLD with a . will not match against a managedzone of the TLD",
			Host: "sub.domain.test.example.co.uk",
			Zones: dns.ZoneList{
				Items: []*dns.Zone{
					{
						ID:      testutil.Pointer("co.uk"),
						DNSName: testutil.Pointer("co.uk"),
					},
				},
			},
			Assert: assertSub("", "", "no valid zone found"),
		},
		{
			name: "no managed zones for host give error",
			Host: "sub.domain.test.example.co.uk",
			Zones: dns.ZoneList{
				Items: []*dns.Zone{},
			},
			Assert: func(t *testing.T, zone *dns.Zone, subdomain string, err error) {
				if err == nil {
					t.Fatalf("expected error, got %v", err)
				}
			},
		},
		{
			name: "should not match when host and zone domain name are identical",
			Host: "test.example.com",
			Zones: dns.ZoneList{
				Items: []*dns.Zone{
					{
						ID:      testutil.Pointer("test.example.com"),
						DNSName: testutil.Pointer("test.example.com"),
					},
				},
			},
			Assert: assertSub("", "", "no valid zone found"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mx, subDomain, err := findMatchingZone(testCase.Host, testCase.Host, testCase.Zones)
			testCase.Assert(t, mx, subDomain, err)
		})
	}
}

func Test_dnsHelper_setEndpoints(t *testing.T) {

	testCases := []struct {
		name      string
		mcgTarget *dns.MultiClusterGatewayTarget
		listener  gatewayapiv1.Listener
		dnsRecord *v1alpha2.DNSRecord
		wantSpec  *v1alpha2.DNSRecordSpec
		wantErr   bool
	}{
		{
			name:     "test wildcard listener weighted",
			listener: getTestListener("*.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{Name: "testgw"},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{
						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{
										{
											Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
											Value: "1.1.1.1",
										},
										{
											Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
											Value: "2.2.2.2",
										},
									},
								},
							},
							ClusterName: "test-cluster-1",
						},
						Geo:    testutil.Pointer(dns.GeoCode("default")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{
										{
											Type:  testutil.Pointer(gatewayapiv1.HostnameAddressType),
											Value: "mylb.example.com",
										},
									},
								},
							},
							ClusterName: "test-cluster-2",
						},
						Geo:    testutil.Pointer(dns.GeoCode("default")),
						Weight: testutil.Pointer(120),
					},
				},
			},
			dnsRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name: "test.example.com",
				},
			},
			wantSpec: &v1alpha2.DNSRecordSpec{
				Endpoints: []*v1alpha2.Endpoint{
					{
						DNSName:    "20qri0.lb-ocnswx.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "default.lb-ocnswx.example.com",
						Targets:       []string{"20qri0.lb-ocnswx.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-ocnswx.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "default.lb-ocnswx.example.com",
						Targets:       []string{"mylb.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "mylb.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "lb-ocnswx.example.com",
						Targets:       []string{"default.lb-ocnswx.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName:    "*.example.com",
						Targets:    []string{"lb-ocnswx.example.com"},
						RecordType: "CNAME",
						RecordTTL:  dns.DefaultCnameTTL,
					},
				},
			},
		},
		{
			name:     "sets geo weighted endpoints wildcard",
			listener: getTestListener("*.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{Name: "testgw"},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{
						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{
										{
											Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
											Value: "1.1.1.1",
										},
										{
											Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
											Value: "2.2.2.2",
										},
									},
								},
							},
							ClusterName: "test-cluster-1",
						},
						Geo:    testutil.Pointer(dns.GeoCode("NA")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{
										{
											Type:  testutil.Pointer(gatewayapiv1.HostnameAddressType),
											Value: "mylb.example.com",
										},
									},
								},
							},
							ClusterName: "test-cluster-2",
						},
						Geo:    testutil.Pointer(dns.GeoCode("IE")),
						Weight: testutil.Pointer(120),
					},
				},
				LoadBalancing: &v1alpha2.LoadBalancingSpec{
					Geo: &v1alpha2.LoadBalancingGeo{
						DefaultGeo: "NA",
					},
				},
			},
			dnsRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name: "gw-test",
				},
			},
			wantSpec: &v1alpha2.DNSRecordSpec{
				Endpoints: []*v1alpha2.Endpoint{
					{
						DNSName:    "20qri0.lb-ocnswx.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "na.lb-ocnswx.example.com",
						Targets:       []string{"20qri0.lb-ocnswx.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-ocnswx.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "ie.lb-ocnswx.example.com",
						Targets:       []string{"mylb.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "mylb.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "lb-ocnswx.example.com",
						Targets:       []string{"na.lb-ocnswx.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName:       "lb-ocnswx.example.com",
						Targets:       []string{"na.lb-ocnswx.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "NA",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "NA",
							},
						},
					},
					{
						DNSName:       "lb-ocnswx.example.com",
						Targets:       []string{"ie.lb-ocnswx.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "IE",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "IE",
							},
						},
					},
					{
						DNSName:    "*.example.com",
						Targets:    []string{"lb-ocnswx.example.com"},
						RecordType: "CNAME",
						RecordTTL:  dns.DefaultCnameTTL,
					},
				},
			},
		},
		{
			name:     "sets weighted endpoints",
			listener: getTestListener("test.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testgw",
						Namespace: "testns",
					},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{
										{
											Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
											Value: "1.1.1.1",
										},
										{
											Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
											Value: "2.2.2.2",
										},
									},
								},
							},
							ClusterName: "test-cluster-1",
						},
						Geo:    testutil.Pointer(dns.GeoCode("default")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{
										{
											Type:  testutil.Pointer(gatewayapiv1.HostnameAddressType),
											Value: "mylb.example.com",
										},
									},
								},
							},
							ClusterName: "test-cluster-2",
						},
						Geo:    testutil.Pointer(dns.GeoCode("default")),
						Weight: testutil.Pointer(120),
					},
				},
			},
			dnsRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name: "test.example.com",
				},
			},
			wantSpec: &v1alpha2.DNSRecordSpec{
				Endpoints: []*v1alpha2.Endpoint{
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
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
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
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
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
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
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
			listener: getTestListener("test.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{Name: "testgw"},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{
										{
											Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
											Value: "1.1.1.1",
										},
										{
											Type:  testutil.Pointer(gatewayapiv1.IPAddressType),
											Value: "2.2.2.2",
										},
									},
								},
							},
							ClusterName: "test-cluster-1",
						},
						Geo:    testutil.Pointer(dns.GeoCode("NA")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{
										{
											Type:  testutil.Pointer(gatewayapiv1.HostnameAddressType),
											Value: "mylb.example.com",
										},
									},
								},
							},
							ClusterName: "test-cluster-2",
						},
						Geo:    testutil.Pointer(dns.GeoCode("IE")),
						Weight: testutil.Pointer(120),
					},
				},
				LoadBalancing: &v1alpha2.LoadBalancingSpec{
					Geo: &v1alpha2.LoadBalancingGeo{
						DefaultGeo: "NA",
					},
				},
			},
			dnsRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name: "test.example.com",
				},
			},
			wantSpec: &v1alpha2.DNSRecordSpec{
				Endpoints: []*v1alpha2.Endpoint{
					{
						DNSName:    "20qri0.lb-ocnswx.test.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "na.lb-ocnswx.test.example.com",
						Targets:       []string{"20qri0.lb-ocnswx.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-ocnswx.test.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "ie.lb-ocnswx.test.example.com",
						Targets:       []string{"mylb.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "mylb.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "lb-ocnswx.test.example.com",
						Targets:       []string{"na.lb-ocnswx.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "*",
							},
						},
					},
					{
						DNSName:       "lb-ocnswx.test.example.com",
						Targets:       []string{"na.lb-ocnswx.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "NA",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "NA",
							},
						},
					},
					{
						DNSName:       "lb-ocnswx.test.example.com",
						Targets:       []string{"ie.lb-ocnswx.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "IE",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha2.ProviderSpecificProperty{
							{
								Name:  "geo-code",
								Value: "IE",
							},
						},
					},
					{
						DNSName:    "test.example.com",
						Targets:    []string{"lb-ocnswx.test.example.com"},
						RecordType: "CNAME",
						RecordTTL:  dns.DefaultCnameTTL,
					},
				},
			},
		},
		{
			name:     "sets no endpoints when no target addresses",
			listener: getTestListener("test.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayapiv1.Gateway{
					ObjectMeta: v1.ObjectMeta{Name: "testgw"},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{},
								},
							},
							ClusterName: "test-cluster-1",
						},
						Geo:    testutil.Pointer(dns.GeoCode("NA")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &utils.ClusterGateway{
							Gateway: gatewayapiv1.Gateway{
								ObjectMeta: v1.ObjectMeta{Name: "testgw"},
								Status: gatewayapiv1.GatewayStatus{
									Addresses: []gatewayapiv1.GatewayStatusAddress{},
								},
							},
							ClusterName: "test-cluster-2",
						},
						Geo:    testutil.Pointer(dns.GeoCode("IE")),
						Weight: testutil.Pointer(120),
					},
				},
				LoadBalancing: &v1alpha2.LoadBalancingSpec{
					Geo: &v1alpha2.LoadBalancingGeo{
						DefaultGeo: "NA",
					},
				},
			},
			dnsRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name: "test.example.com",
				},
			},
			wantSpec: &v1alpha2.DNSRecordSpec{
				Endpoints: []*v1alpha2.Endpoint{},
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(testCase.dnsRecord).Build()
			s := dnsHelper{Client: f}
			if err := s.setEndpoints(context.TODO(), testCase.mcgTarget, testCase.dnsRecord, testCase.listener, v1alpha2.LoadBalancedRoutingStrategy); (err != nil) != testCase.wantErr {
				t.Errorf("SetEndpoints() error = %v, wantErr %v", err, testCase.wantErr)
			}

			gotRecord := &v1alpha2.DNSRecord{}
			if err := f.Get(context.TODO(), client.ObjectKeyFromObject(testCase.dnsRecord), gotRecord); err != nil {
				t.Errorf("error gettinging updated DNSrecord")
			} else {

				sort.Slice(gotRecord.Spec.Endpoints, func(i, j int) bool {
					id1 := gotRecord.Spec.Endpoints[i].DNSName + gotRecord.Spec.Endpoints[i].SetIdentifier
					id2 := gotRecord.Spec.Endpoints[j].DNSName + gotRecord.Spec.Endpoints[j].SetIdentifier
					return id1 < id2
				})

				sort.Slice(testCase.wantSpec.Endpoints, func(i, j int) bool {
					id1 := testCase.wantSpec.Endpoints[i].DNSName + testCase.wantSpec.Endpoints[i].SetIdentifier
					id2 := testCase.wantSpec.Endpoints[j].DNSName + testCase.wantSpec.Endpoints[j].SetIdentifier
					return id1 < id2
				})

				if !equality.Semantic.DeepEqual(gotRecord.Spec.Endpoints, testCase.wantSpec.Endpoints) {
					t.Errorf("SetEndpoints() updated DNSRecord spec: \n%v, want spec: \n%v", gotRecord.Spec, *testCase.wantSpec)
				}
			}

		})
	}
}

func Test_dnsHelper_getDNSRecordForListener(t *testing.T) {
	testCases := []struct {
		name      string
		Listener  gatewayapiv1.Listener
		Assert    func(t *testing.T, err error)
		DNSRecord *v1alpha2.DNSRecord
		Gateway   *gatewayapiv1.Gateway
		DNSPolicy *v1alpha2.DNSPolicy
	}{
		{
			name:     "test get dns record returns record",
			Listener: getTestListener("a.b.c.com"),
			DNSRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "gw-test",
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						"gateway-namespace":               "test",
						"gateway":                         "tstgateway",
					},
				},
			},
			Gateway: &gatewayapiv1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					UID:       types.UID("test"),
					Name:      "gw",
					Namespace: "test",
				},
			},

			Assert: testutil.AssertError(""),
		},
		{
			name: "test get dns error when not found",
			DNSRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "gw-test",
					Namespace: "test",
				},
			},
			Gateway: &gatewayapiv1.Gateway{},
			Assert:  testutil.AssertError("not found"),
		},
		{
			name: "test get dns error when referencing different Gateway",
			DNSRecord: &v1alpha2.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "gw-test",
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "different-tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						"gateway-namespace":               "test",
						"gateway":                         "different-gateway",
					},
				},
			},
			Gateway: &gatewayapiv1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					UID:       types.UID("test"),
					Name:      "other",
					Namespace: "test",
				},
			},
			Assert: testutil.AssertError("not found"),
		},
		{
			name:     "test get dns error when not owned by Gateway",
			Listener: getTestListener("other.com"),
			DNSRecord: &v1alpha2.DNSRecord{
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
			Gateway: &gatewayapiv1.Gateway{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstgateway",
					Namespace: "test",
				},
			},
			DNSPolicy: &v1alpha2.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "tstpolicy",
					Namespace: "test",
				},
			},
			Assert: testutil.AssertError("not found"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testutil.GetValidTestScheme()).WithObjects(testCase.DNSRecord).Build()
			s := &dnsHelper{Client: f}
			_, err := s.getDNSRecordForListener(context.TODO(), testCase.Listener, testCase.Gateway)
			testCase.Assert(t, err)
		})
	}

}

func assertSub(domain string, subdomain string, err string) func(t *testing.T, expectedzone *dns.Zone, expectedsubdomain string, expectedErr error) {
	return func(t *testing.T, expectedzone *dns.Zone, expectedsubdomain string, expectedErr error) {
		if (err == "") != (expectedErr == nil) {
			t.Errorf("expected error '%s' but got '%s'", err, expectedErr)
		}
		if expectedErr != nil && !strings.Contains(expectedErr.Error(), err) {
			t.Errorf("expected error to be '%s' but got '%s'", err, expectedErr)
		}
		if subdomain != expectedsubdomain {
			t.Fatalf("expected subdomain '%v', got '%v'", subdomain, expectedsubdomain)
		}
		if expectedzone != nil && domain != *expectedzone.DNSName {
			t.Fatalf("expected zone with domain name '%v', got '%v'", domain, expectedzone)
		}
		if expectedzone == nil && domain != "" {
			t.Fatalf("expected zone to be '%v', got '%v'", domain, expectedzone)
		}
	}
}
