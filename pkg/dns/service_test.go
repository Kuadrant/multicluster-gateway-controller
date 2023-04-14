package dns_test

import (
	"context"
	"testing"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/apis/v1alpha1"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeHostResolver struct{}

func (fr *fakeHostResolver) LookupIPAddr(ctx context.Context, host string) ([]dns.HostAddress, error) {
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

			f := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.DNSRecord).Build()
			s := dns.NewService(f, tc.Resolver())
			record, err := s.GetDNSRecord(context.TODO(), tc.SubDomain, tc.MZ())
			tc.Assert(t, record, err)
		})
	}

}
