//go:build unit

package dnspolicy

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"

	"k8s.io/apimachinery/pkg/api/equality"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
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

func getTestListener(hostName string) gatewayv1beta1.Listener {
	host := gatewayv1beta1.Hostname(hostName)
	return gatewayv1beta1.Listener{
		Name:     "test",
		Hostname: &host,
	}
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

func Test_dnsHelper_createDNSRecordForListener(t *testing.T) {
	var (
		testGatewayName  = "tstgateway"
		testListenerName = "test"
	)
	type args struct {
		gateway     *gatewayv1beta1.Gateway
		dnsPolicy   *v1alpha1.DNSPolicy
		managedZone *v1alpha1.ManagedZone
		listener    gatewayv1beta1.Listener
	}
	testCases := []struct {
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
						Name:      testGatewayName,
						Namespace: "test",
					},
				},
				listener: getTestListener("test.domain.com"),
				dnsPolicy: &v1alpha1.DNSPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstpolicy",
						Namespace: "test",
					},
				},
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
					Name:      dnsRecordName(testGatewayName, testListenerName),
					Namespace: "test",
					Labels: map[string]string{
						"kuadrant.io/dnspolicy":           "tstpolicy",
						"kuadrant.io/dnspolicy-namespace": "test",
						LabelGatewayNSRef:                 "test",
						LabelGatewayReference:             "tstgateway",
						LabelListenerReference:            testListenerName,
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
						Name:      testGatewayName,
						Namespace: "test",
					},
				},
				listener: getTestListener("test.domain.com"),
				dnsPolicy: &v1alpha1.DNSPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstpolicy",
						Namespace: "test",
					},
				},
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
							Name:      dnsRecordName(testGatewayName, testListenerName),
							Namespace: "test",
						},
					},
				},
			},
			wantRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:            dnsRecordName(testGatewayName, testListenerName),
					Namespace:       "test",
					ResourceVersion: "999",
				},
				TypeMeta: v1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "kuadrant.io/v1alpha1",
				},
			},
		},
		{
			name: "DNS record for wildcard listener",
			args: args{
				gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      testGatewayName,
						Namespace: "test",
					},
				},
				listener: getTestListener("*.domain.com"),
				dnsPolicy: &v1alpha1.DNSPolicy{
					ObjectMeta: v1.ObjectMeta{
						Name:      "tstpolicy",
						Namespace: "test",
					},
				},
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
				Items: []v1alpha1.DNSRecord{},
			},
			wantRecord: &v1alpha1.DNSRecord{
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
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithLists(testCase.recordList).Build()
			s := dnsHelper{Client: f}

			gotRecord, err := s.createDNSRecordForListener(context.TODO(), testCase.args.gateway, testCase.args.dnsPolicy, testCase.args.managedZone, testCase.args.listener)
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

func Test_dnsHelper_findMatchingManagedZone(t *testing.T) {
	testCases := []struct {
		name   string
		Host   string
		Zones  []v1alpha1.ManagedZone
		Assert func(t *testing.T, zone *v1alpha1.ManagedZone, subdomain string, err error)
	}{
		{
			name: "finds the matching managed zone",
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
			name: "finds the most exactly matching managed zone",
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
			name: "returns a single subdomain",
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
			name: "returns an error when nothing matches",
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
			name: "handles TLD with a dot",
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
			name: "TLD with a . will not match against a managedzone of the TLD",
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
			name:  "no managed zones for host give error",
			Host:  "sub.domain.test.example.co.uk",
			Zones: []v1alpha1.ManagedZone{},
			Assert: func(t *testing.T, zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if err == nil {
					t.Fatalf("expected error, got %v", err)
				}
			},
		},
		{
			name: "should not match when host and zone domain name are identical",
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

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			mx, subDomain, err := findMatchingManagedZone(testCase.Host, testCase.Host, testCase.Zones)
			testCase.Assert(t, mx, subDomain, err)
		})
	}
}

func Test_dnsHelper_setEndpoints(t *testing.T) {

	testCases := []struct {
		name      string
		mcgTarget *dns.MultiClusterGatewayTarget
		listener  gatewayv1beta1.Listener
		dnsRecord *v1alpha1.DNSRecord
		dnsPolicy *v1alpha1.DNSPolicy
		probeOne  *v1alpha1.DNSHealthCheckProbe
		probeTwo  *v1alpha1.DNSHealthCheckProbe
		wantSpec  *v1alpha1.DNSRecordSpec
		wantErr   bool
	}{
		{
			name:     "test wildcard listener weighted",
			listener: getTestListener("*.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayv1beta1.Gateway{},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{
						ClusterGateway: &dns.ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-2",
								},
							},
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
			dnsPolicy: &v1alpha1.DNSPolicy{},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testOne",
					Namespace: "namespace",
				},
			},
			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testTwo",
					Namespace: "namespace",
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-oe3k96.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "default.lb-oe3k96.example.com",
						Targets:       []string{"20qri0.lb-oe3k96.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-oe3k96.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "default.lb-oe3k96.example.com",
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
						DNSName:       "lb-oe3k96.example.com",
						Targets:       []string{"default.lb-oe3k96.example.com"},
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
						DNSName:    "*.example.com",
						Targets:    []string{"lb-oe3k96.example.com"},
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
				Gateway: &gatewayv1beta1.Gateway{},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &dns.ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-2",
								},
							},
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
					Name: "gw-test",
				},
			},
			dnsPolicy: &v1alpha1.DNSPolicy{},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testOne",
					Namespace: "namespace",
				},
			},
			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testTwo",
					Namespace: "namespace",
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-oe3k96.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "na.lb-oe3k96.example.com",
						Targets:       []string{"20qri0.lb-oe3k96.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-oe3k96.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "ie.lb-oe3k96.example.com",
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
						DNSName:       "lb-oe3k96.example.com",
						Targets:       []string{"na.lb-oe3k96.example.com"},
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
						DNSName:       "lb-oe3k96.example.com",
						Targets:       []string{"na.lb-oe3k96.example.com"},
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
						DNSName:       "lb-oe3k96.example.com",
						Targets:       []string{"ie.lb-oe3k96.example.com"},
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
						DNSName:    "*.example.com",
						Targets:    []string{"lb-oe3k96.example.com"},
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
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: v1.ObjectMeta{
						Name:      "testgw",
						Namespace: "testns",
					},
				},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &dns.ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-2",
								},
							},
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
			dnsPolicy: &v1alpha1.DNSPolicy{},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testOne",
					Namespace: "namespace",
				},
			},
			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testTwo",
					Namespace: "namespace",
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
			listener: getTestListener("test.example.com"),
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayv1beta1.Gateway{},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &dns.ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-2",
								},
							},
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
			dnsPolicy: &v1alpha1.DNSPolicy{},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testOne",
					Namespace: "namespace",
				},
			},
			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testTwo",
					Namespace: "namespace",
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-oe3k96.test.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "na.lb-oe3k96.test.example.com",
						Targets:       []string{"20qri0.lb-oe3k96.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-oe3k96.test.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "ie.lb-oe3k96.test.example.com",
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
						DNSName:       "lb-oe3k96.test.example.com",
						Targets:       []string{"na.lb-oe3k96.test.example.com"},
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
						DNSName:       "lb-oe3k96.test.example.com",
						Targets:       []string{"na.lb-oe3k96.test.example.com"},
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
						DNSName:       "lb-oe3k96.test.example.com",
						Targets:       []string{"ie.lb-oe3k96.test.example.com"},
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
						Targets:    []string{"lb-oe3k96.test.example.com"},
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
				Gateway: &gatewayv1beta1.Gateway{},
				ClusterGatewayTargets: []dns.ClusterGatewayTarget{
					{

						ClusterGateway: &dns.ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{},
						},
						Geo:    testutil.Pointer(dns.GeoCode("NA")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &dns.ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-2",
								},
							},
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
			dnsPolicy: &v1alpha1.DNSPolicy{},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testOne",
					Namespace: "namespace",
				},
			},
			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testTwo",
					Namespace: "namespace",
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{},
			},
		},
		{
			name:     "test endpoint presence when probe is present but no health status is reported yet",
			listener: getTestListener("*.example.com"),
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-2",
								},
							},
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
			dnsPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testpolicy",
					Namespace: "testns",
				},
			},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Labels: commonDNSRecordLabels(
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testgw"},
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testpolicy",
						}),
					Name:      "1.1.1.1-test.example.com",
					Namespace: "testns",
				},
			},
			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Labels: commonDNSRecordLabels(
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testgw"},
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testpolicy",
						}),
					Name:      "2.2.2.2-test.example.com",
					Namespace: "testns",
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-0ecjaw.example.com",
						Targets:    []string{"1.1.1.1", "2.2.2.2"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:       "default.lb-0ecjaw.example.com",
						Targets:       []string{"20qri0.lb-0ecjaw.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "20qri0.lb-0ecjaw.example.com",
						RecordTTL:     dns.DefaultTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "weight",
								Value: "120",
							},
						},
					},
					{
						DNSName:       "default.lb-0ecjaw.example.com",
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
						DNSName:       "lb-0ecjaw.example.com",
						Targets:       []string{"default.lb-0ecjaw.example.com"},
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
						DNSName:    "*.example.com",
						Targets:    []string{"lb-0ecjaw.example.com"},
						RecordType: "CNAME",
						RecordTTL:  dns.DefaultCnameTTL,
					},
				},
			},
		},
		{
			name:     "test endpoint presence when probe is present but healthy status is reported",
			listener: getTestListener("test.example.com"),
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "1.1.1.1",
								},
							},
						},
						Geo:    testutil.Pointer(dns.GeoCode("default")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &dns.ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "2.2.2.2",
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
			dnsPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testpolicy",
					Namespace: "testns",
				},
			},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Labels: commonDNSRecordLabels(
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testgw"},
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testpolicy",
						}),
					Name:      "1.1.1.1-test.example.com",
					Namespace: "testns",
				},
				Status: v1alpha1.DNSHealthCheckProbeStatus{
					Healthy: aws.Bool(true),
				},
			},

			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Labels: commonDNSRecordLabels(
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testgw"},
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testpolicy",
						}),
					Name:      "2.2.2.2-test.example.com",
					Namespace: "testns",
				},
				Status: v1alpha1.DNSHealthCheckProbeStatus{
					Healthy: aws.Bool(true),
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-0ecjaw.test.example.com",
						Targets:    []string{"1.1.1.1"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:    "20qri0.lb-0ecjaw.test.example.com",
						Targets:    []string{"2.2.2.2"},
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
			name:     "test removal of endpoint based on probe",
			listener: getTestListener("test.example.com"),
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-2",
								},
							},
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "1.1.1.1",
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
			dnsPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testpolicy",
					Namespace: "testns",
				},
			},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Labels: commonDNSRecordLabels(
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testgw"},
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testpolicy",
						}),
					Name:      "1.1.1.1-test.example.com",
					Namespace: "testns",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					FailureThreshold: aws.Int(4),
				},
				Status: v1alpha1.DNSHealthCheckProbeStatus{
					Healthy:             aws.Bool(false),
					ConsecutiveFailures: 54,
				},
			},
			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Labels: commonDNSRecordLabels(
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testgw"},
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testpolicy",
						}),
					Name:      "2.2.2.2-test.example.com",
					Namespace: "testns",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					FailureThreshold: aws.Int(4),
				},
				Status: v1alpha1.DNSHealthCheckProbeStatus{
					Healthy:             aws.Bool(false),
					ConsecutiveFailures: 2,
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-0ecjaw.test.example.com",
						Targets:    []string{"2.2.2.2"},
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
			name:     "test no removal of endpoint when all probes are unhealthy",
			listener: getTestListener("test.example.com"),
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
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-1",
								},
							},
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "1.1.1.1",
								},
							},
						},
						Geo:    testutil.Pointer(dns.GeoCode("default")),
						Weight: testutil.Pointer(120),
					},
					{

						ClusterGateway: &dns.ClusterGateway{
							Cluster: &testutil.TestResource{
								ObjectMeta: v1.ObjectMeta{
									Name: "test-cluster-2",
								},
							},
							GatewayAddresses: []gatewayv1beta1.GatewayAddress{
								{
									Type:  testutil.Pointer(gatewayv1beta1.IPAddressType),
									Value: "2.2.2.2",
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
			dnsPolicy: &v1alpha1.DNSPolicy{
				ObjectMeta: v1.ObjectMeta{
					Name:      "testpolicy",
					Namespace: "testns",
				},
			},
			probeOne: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Labels: commonDNSRecordLabels(
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testgw"},
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testpolicy",
						}),
					Name:      "1.1.1.1-test.example.com",
					Namespace: "testns",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					FailureThreshold: aws.Int(4),
				},
				Status: v1alpha1.DNSHealthCheckProbeStatus{
					Healthy:             aws.Bool(false),
					ConsecutiveFailures: 6,
				},
			},

			probeTwo: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: v1.ObjectMeta{
					Labels: commonDNSRecordLabels(
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testgw"},
						types.NamespacedName{
							Namespace: "testns",
							Name:      "testpolicy",
						}),
					Name:      "2.2.2.2-test.example.com",
					Namespace: "testns",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					FailureThreshold: aws.Int(4),
				},
				Status: v1alpha1.DNSHealthCheckProbeStatus{
					Healthy:             aws.Bool(false),
					ConsecutiveFailures: 6,
				},
			},
			wantSpec: &v1alpha1.DNSRecordSpec{
				Endpoints: []*v1alpha1.Endpoint{
					{
						DNSName:    "20qri0.lb-0ecjaw.test.example.com",
						Targets:    []string{"1.1.1.1"},
						RecordType: "A",
						RecordTTL:  dns.DefaultTTL,
					},
					{
						DNSName:    "2pj3we.lb-0ecjaw.test.example.com",
						Targets:    []string{"2.2.2.2"},
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
						Targets:       []string{"2pj3we.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "2pj3we.lb-0ecjaw.test.example.com",
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
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(testCase.dnsRecord, testCase.probeOne, testCase.probeTwo).Build()
			s := dnsHelper{Client: f}
			if err := s.setEndpoints(context.TODO(), testCase.mcgTarget, testCase.dnsRecord, testCase.dnsPolicy, testCase.listener); (err != nil) != testCase.wantErr {
				t.Errorf("SetEndpoints() error = %v, wantErr %v", err, testCase.wantErr)
			}

			gotRecord := &v1alpha1.DNSRecord{}
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
		Listener  gatewayv1beta1.Listener
		Assert    func(t *testing.T, err error)
		DNSRecord *v1alpha1.DNSRecord
		Gateway   *gatewayv1beta1.Gateway
		DNSPolicy *v1alpha1.DNSPolicy
	}{
		{
			name:     "test get dns record returns record",
			Listener: getTestListener("a.b.c.com"),
			DNSRecord: &v1alpha1.DNSRecord{
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
			Gateway: &gatewayv1beta1.Gateway{
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
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: v1.ObjectMeta{
					Name:      "gw-test",
					Namespace: "test",
				},
			},
			Gateway: &gatewayv1beta1.Gateway{},
			Assert:  testutil.AssertError("not found"),
		},
		{
			name: "test get dns error when referencing different Gateway",
			DNSRecord: &v1alpha1.DNSRecord{
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
			Gateway: &gatewayv1beta1.Gateway{
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

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testutil.GetValidTestScheme()).WithObjects(testCase.DNSRecord).Build()
			s := &dnsHelper{Client: f}
			_, err := s.getDNSRecordForListener(context.TODO(), testCase.Listener, testCase.Gateway)
			testCase.Assert(t, err)
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
