//go:build unit

package dns_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

func TestDNS_GetDNSRecords(t *testing.T) {
	cases := []struct {
		Name      string
		MZ        func() *v1alpha1.ManagedZone
		SubDomain string
		Assert    func(t *testing.T, dnsRecord *v1alpha1.DNSRecord, err error)
		DNSRecord *v1alpha1.DNSRecord
		Gateway   *gatewayv1beta1.Gateway
	}{
		{
			Name: "test get dns record returns record",
			MZ: func() *v1alpha1.ManagedZone {
				return &v1alpha1.ManagedZone{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "b.c.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "b.c.com",
					},
				}
			},
			SubDomain: "a",
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a.b.c.com",
					Namespace: "test",
					Labels: map[string]string{
						dns.LabelGatewayReference: "test",
					},
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("test"),
				},
			},

			Assert: func(t *testing.T, dnsRecord *v1alpha1.DNSRecord, err error) {
				if err != nil {
					t.Fatalf("expectd no error but got %s", err)
				}
			},
		},
		{
			Name: "test get dns error when not found",
			MZ: func() *v1alpha1.ManagedZone {
				return &v1alpha1.ManagedZone{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "b.c.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "b.c.com",
					},
				}
			},
			SubDomain: "a",
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other.com",
					Namespace: "test",
				},
			},
			Gateway: &gatewayv1beta1.Gateway{},
			Assert: func(t *testing.T, dnsRecord *v1alpha1.DNSRecord, err error) {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				if !k8serrors.IsNotFound(err) {
					t.Fatalf("expected a not found error but got %s", err)
				}
			},
		},
		{
			Name: "test get dns error when referencing different Gateway",
			MZ: func() *v1alpha1.ManagedZone {
				return &v1alpha1.ManagedZone{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "b.c.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "b.c.com",
					},
				}
			},
			SubDomain: "a",
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "a.b.c.com",
					Namespace: "test",
					Labels: map[string]string{
						dns.LabelGatewayReference: "reference",
					},
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("test"),
				},
			},
			Assert: func(t *testing.T, dnsRecord *v1alpha1.DNSRecord, err error) {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				if k8serrors.IsNotFound(err) {
					t.Fatalf("expected a custom error but got %s", err)
				}
			},
		},
		{
			Name: "test get dns error when not owned by Gateway",
			MZ: func() *v1alpha1.ManagedZone {
				return &v1alpha1.ManagedZone{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "b.c.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "b.c.com",
					},
				}
			},
			SubDomain: "a",
			DNSRecord: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "other.com",
					Namespace: "test",
					Labels: map[string]string{
						dns.LabelGatewayReference: "test",
					},
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("not"),
				},
			},
			Assert: func(t *testing.T, dnsRecord *v1alpha1.DNSRecord, err error) {
				if err == nil {
					t.Fatalf("expected an error but got none")
				}
				if !k8serrors.IsNotFound(err) {
					t.Fatalf("expected a not found error but got %s", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			gw := traffic.NewGateway(tc.Gateway)
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(tc.DNSRecord).Build()
			s := dns.NewService(f)
			record, err := s.GetDNSRecord(context.TODO(), tc.SubDomain, tc.MZ(), gw)
			tc.Assert(t, record, err)
		})
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

func TestDNS_findMatchingManagedZone(t *testing.T) {
	cases := []struct {
		Name   string
		Host   string
		Zones  []v1alpha1.ManagedZone
		Assert func(zone *v1alpha1.ManagedZone, subdomain string, err error)
	}{
		{
			Name: "finds the matching managed zone",
			Host: "sub.domain.test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "example.com",
					},
				},
			},
			Assert: func(zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if subdomain != "sub.domain.test" {
					t.Fatalf("expected subdomain 'sub.domain.test', got '%v'", subdomain)
				}
				if zone.Spec.DomainName != "example.com" {
					t.Fatalf("expected zone with domain name 'example.com' got %v", zone.Spec.DomainName)
				}
			},
		},
		{
			Name: "finds the most exactly matching managed zone",
			Host: "sub.domain.test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "example.com",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "test.example.com",
					},
				},
			},
			Assert: func(zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if zone.Spec.DomainName != "test.example.com" {
					t.Fatalf("expected found zone to be the longest matching zone, expected test.example.com, got %v", zone.Spec.DomainName)
				}

				if subdomain != "sub.domain" {
					t.Fatalf("expected subdomain 'sub.domain', got '%v'", subdomain)
				}
			},
		},
		{
			Name: "returns a single subdomain",
			Host: "sub.test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test.example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "test.example.com",
					},
				},
			},
			Assert: func(zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if zone.Spec.DomainName != "test.example.com" {
					t.Fatalf("expected found zone to be the longest matching zone, expected test.example.com, got %v", zone.Spec.DomainName)
				}

				if subdomain != "sub" {
					t.Fatalf("expected subdomain 'sub', got '%v'", subdomain)
				}
			},
		},
		{
			Name: "returns an error when nothing matches",
			Host: "sub.test.example.com",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "testing.example.com",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "testing.example.com",
					},
				},
			},
			Assert: func(zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if zone != nil {
					t.Fatalf("expected no zone to match, got: %v", zone.Name)
				}

				if subdomain != "" {
					t.Fatalf("expected subdomain '', got '%v'", subdomain)
				}

				if err == nil {
					t.Fatal("expected an error, got nil")
				}
			},
		},
		{
			Name: "handles TLD with a dot",
			Host: "sub.domain.test.example.co.uk",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "example.co.uk",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "example.co.uk",
					},
				},
			},
			Assert: func(zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}

				if subdomain != "sub.domain.test" {
					t.Fatalf("expected subdomain 'sub.domain.test', got '%v'", subdomain)
				}
				if zone.Spec.DomainName != "example.co.uk" {
					t.Fatalf("expected zone with domain name 'example.co.uk' got %v", zone.Spec.DomainName)
				}
			},
		},
		{
			Name: "TLD with a . will not match against a managedzone of the TLD",
			Host: "sub.domain.test.example.co.uk",
			Zones: []v1alpha1.ManagedZone{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "co.uk",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "co.uk",
					},
				},
			},
			Assert: func(zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if err == nil {
					t.Fatalf("expected error, got %v", err)
				}

				if subdomain != "" {
					t.Fatalf("expected subdomain '', got '%v'", subdomain)
				}
				if zone != nil {
					t.Fatalf("expected zone to be nil, got %v", zone.Name)
				}
			},
		},
		{
			Name:  "no managed zones for host give error",
			Host:  "sub.domain.test.example.co.uk",
			Zones: []v1alpha1.ManagedZone{},
			Assert: func(zone *v1alpha1.ManagedZone, subdomain string, err error) {
				if err == nil {
					t.Fatalf("expected error, got %v", err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Assert(dns.FindMatchingManagedZone(tc.Host, tc.Host, tc.Zones))
		})
	}
}

func TestService_CleanupDNSRecords(t *testing.T) {

	tests := []struct {
		name    string
		gateway *gatewayv1beta1.Gateway
		record  *v1alpha1.DNSRecord
		wantErr bool
	}{
		{
			name: "DNS record gets deleted",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("test"),
				},
			},
			record: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						dns.LabelGatewayReference: "test",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "no DNS records do be deleted",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("test"),
				},
			},
			record:  &v1alpha1.DNSRecord{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gw := traffic.NewGateway(tt.gateway)
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(tt.record).Build()
			s := dns.NewService(f)
			if err := s.CleanupDNSRecords(context.TODO(), gw); (err != nil) != tt.wantErr {
				t.Errorf("CleanupDNSRecords() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestService_GetManagedZoneForHost(t *testing.T) {
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
			host: "example.com",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
				},
			},
			mz: &v1alpha1.ManagedZoneList{
				Items: []v1alpha1.ManagedZone{
					{
						ObjectMeta: metav1.ObjectMeta{
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
			wantSubdomain: "example.com",
			wantErr:       false,
		},
		{
			name: "unable to list MZ",
			host: "example.com",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
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
			s := dns.NewService(f)

			gotMZ, gotSubdomain, err := s.GetManagedZoneForHost(context.TODO(), tt.host, gw)
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

func TestService_SetEndpoints(t *testing.T) {

	tests := []struct {
		name      string
		mcgTarget *dns.MultiClusterGatewayTarget
		dnsRecord *v1alpha1.DNSRecord
		wantSpec  *v1alpha1.DNSRecordSpec
		wantErr   bool
	}{
		{
			name: "sets weighted endpoints",
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
								Name:  "geo-country-code",
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
			name: "sets geo weighted endpoints",
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
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
						Geo:    testutil.Pointer(dns.GeoCode("C-NA")),
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
						DefaultGeo: "C-NA",
					},
				},
			},
			dnsRecord: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
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
						DNSName:       "c-na.lb-0ecjaw.test.example.com",
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
						Targets:       []string{"c-na.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "default",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "geo-country-code",
								Value: "*",
							},
						},
					},
					{
						DNSName:       "lb-0ecjaw.test.example.com",
						Targets:       []string{"c-na.lb-0ecjaw.test.example.com"},
						RecordType:    "CNAME",
						SetIdentifier: "C-NA",
						RecordTTL:     dns.DefaultCnameTTL,
						ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
							{
								Name:  "geo-continent-code",
								Value: "C-NA",
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
								Name:  "geo-country-code",
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
			name: "sets no endpoints when no target addresses",
			mcgTarget: &dns.MultiClusterGatewayTarget{
				Gateway: &gatewayv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
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
						Geo:    testutil.Pointer(dns.GeoCode("C-NA")),
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
						DefaultGeo: "C-NA",
					},
				},
			},
			dnsRecord: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
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
			s := dns.NewService(f)

			if err := s.SetEndpoints(context.TODO(), tt.mcgTarget, tt.dnsRecord); (err != nil) != tt.wantErr {
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

func TestService_CreateDNSRecord(t *testing.T) {
	type args struct {
		subDomain   string
		managedZone *v1alpha1.ManagedZone
		owner       metav1.Object
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
				subDomain: "sub",
				managedZone: &v1alpha1.ManagedZone{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mz",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "domain.com",
					},
				},
				owner: &gatewayv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("gatewayUID"),
					},
				},
			},
			recordList: &v1alpha1.DNSRecordList{},
			wantRecord: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sub.domain.com",
					Namespace: "test",
					Labels: map[string]string{
						dns.LabelRecordID:         "sub",
						dns.LabelGatewayReference: "gatewayUID",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "gateway.networking.k8s.io/v1beta1",
							Kind:       "Gateway",
							UID:        types.UID("gatewayUID"),
						},
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
				subDomain: "sub",
				managedZone: &v1alpha1.ManagedZone{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "mz",
						Namespace: "test",
					},
					Spec: v1alpha1.ManagedZoneSpec{
						DomainName: "domain.com",
					},
				},
				owner: &gatewayv1beta1.Gateway{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID("gatewayUID"),
					},
				},
			},
			recordList: &v1alpha1.DNSRecordList{
				Items: []v1alpha1.DNSRecord{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "sub.domain.com",
							Namespace: "test",
						},
					},
				},
			},
			wantRecord: &v1alpha1.DNSRecord{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "sub.domain.com",
					Namespace:       "test",
					ResourceVersion: "999",
				},
				TypeMeta: metav1.TypeMeta{
					Kind:       "DNSRecord",
					APIVersion: "kuadrant.io/v1alpha1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithLists(tt.recordList).Build()
			s := dns.NewService(f)

			gotRecord, err := s.CreateDNSRecord(context.TODO(), tt.args.subDomain, tt.args.managedZone, tt.args.owner)
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

func TestService_GetManagedHosts(t *testing.T) {
	tests := []struct {
		name      string
		gateway   *gatewayv1beta1.Gateway
		initLists []client.ObjectList
		want      []v1alpha1.ManagedHost
		wantErr   bool
	}{
		{
			name: "got managed hosts",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					UID:       types.UID("gatewayUID"),
				},
				Spec: gatewayv1beta1.GatewaySpec{
					Listeners: []gatewayv1beta1.Listener{
						{
							Hostname: testutil.Pointer(gatewayv1beta1.Hostname("sub.domain.com")),
						},
					},
				},
			},
			initLists: []client.ObjectList{
				&v1alpha1.ManagedZoneList{
					Items: []v1alpha1.ManagedZone{
						{
							ObjectMeta: metav1.ObjectMeta{
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
							ObjectMeta: metav1.ObjectMeta{
								Name:      "sub.domain.com",
								Namespace: "test",
								Labels: map[string]string{
									dns.LabelGatewayReference: "gatewayUID",
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
						ObjectMeta: metav1.ObjectMeta{
							Namespace:       "test",
							ResourceVersion: "999",
						},
						Spec: v1alpha1.ManagedZoneSpec{
							DomainName: "domain.com",
						},
					},
					DnsRecord: &v1alpha1.DNSRecord{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "sub.domain.com",
							Namespace:       "test",
							ResourceVersion: "999",
							Labels: map[string]string{
								dns.LabelGatewayReference: "gatewayUID",
							},
						},
						TypeMeta: metav1.TypeMeta{
							Kind:       "DNSRecord",
							APIVersion: "kuadrant.io/v1alpha1",
						},
					},
				},
			},
		},
		{
			name: "No hosts retrieved for CNAME or externaly managed host",
			gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					UID:       types.UID("gatewayUID"),
				},
				Spec: gatewayv1beta1.GatewaySpec{
					Listeners: []gatewayv1beta1.Listener{
						{
							Hostname: testutil.Pointer(gatewayv1beta1.Hostname("sub.domain.com")),
						},
					},
				},
			},
			initLists: []client.ObjectList{},
			want:      []v1alpha1.ManagedHost{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithLists(tt.initLists...).Build()
			s := dns.NewService(f)

			got, err := s.GetManagedHosts(context.TODO(), traffic.NewGateway(tt.gateway))
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
