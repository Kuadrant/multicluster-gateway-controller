//go:build unit

package dns_test

import (
	"context"
	"testing"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/dns"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/traffic"
)

type fakeHostResolver struct{}

func (fr *fakeHostResolver) LookupIPAddr(_ context.Context, _ string) ([]dns.HostAddress, error) {
	return nil, nil
}

func TestDNS_GetDNSRecords(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("falied to add work scheme %s ", err)
	}
	cases := []struct {
		Name      string
		Resolver  func() dns.HostResolver
		MZ        func() *v1alpha1.ManagedZone
		SubDomain string
		Assert    func(t *testing.T, dnsRecord *v1alpha1.DNSRecord, err error)
		DNSRecord *v1alpha1.DNSRecord
		Gateway   *gatewayv1beta1.Gateway
	}{
		{
			Name: "test get dns record returns record",
			Resolver: func() dns.HostResolver {
				return &fakeHostResolver{}
			},
			MZ: func() *v1alpha1.ManagedZone {
				return &v1alpha1.ManagedZone{
					ObjectMeta: v1.ObjectMeta{
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
				ObjectMeta: v1.ObjectMeta{
					Name:      "a.b.c.com",
					Namespace: "test",
					Labels: map[string]string{
						dns.LabelGatewayReference: "test",
					},
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
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
			Resolver: func() dns.HostResolver {
				return &fakeHostResolver{}
			},
			MZ: func() *v1alpha1.ManagedZone {
				return &v1alpha1.ManagedZone{
					ObjectMeta: v1.ObjectMeta{
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
				ObjectMeta: v1.ObjectMeta{
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
			Name: "test get dns error when not owned by gateway",
			Resolver: func() dns.HostResolver {
				return &fakeHostResolver{}
			},
			MZ: func() *v1alpha1.ManagedZone {
				return &v1alpha1.ManagedZone{
					ObjectMeta: v1.ObjectMeta{
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
				ObjectMeta: v1.ObjectMeta{
					Name:      "other.com",
					Namespace: "test",
					Labels: map[string]string{
						dns.LabelGatewayReference: "test",
					},
				},
			},
			Gateway: &gatewayv1beta1.Gateway{
				ObjectMeta: v1.ObjectMeta{
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
			fp := &dns.FakeProvider{}
			gw := traffic.NewGateway(tc.Gateway)
			f := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.DNSRecord).Build()
			s := dns.NewService(f, tc.Resolver(), fp)
			record, err := s.GetDNSRecord(context.TODO(), tc.SubDomain, tc.MZ(), gw)
			tc.Assert(t, record, err)
		})
	}

}

func TestSetProviderSpecific(t *testing.T) {
	endpoint := &v1alpha1.Endpoint{
		ProviderSpecific: []v1alpha1.ProviderSpecificProperty{
			{Name: "aws/weight", Value: "120"},
		},
	}

	// Test updating an existing property
	endpoint.SetProviderSpecific("aws/weight", "60")
	for _, property := range endpoint.ProviderSpecific {
		if property.Name == "aws/weight" && property.Value != "60" {
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
					ObjectMeta: v1.ObjectMeta{
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
					ObjectMeta: v1.ObjectMeta{
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
					ObjectMeta: v1.ObjectMeta{
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
					ObjectMeta: v1.ObjectMeta{
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
					ObjectMeta: v1.ObjectMeta{
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
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			tc.Assert(dns.FindMatchingManagedZone(tc.Host, tc.Host, tc.Zones))
		})
	}
}
