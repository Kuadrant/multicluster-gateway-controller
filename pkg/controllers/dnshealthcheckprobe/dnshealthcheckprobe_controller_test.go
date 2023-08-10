package dnshealthcheckprobe

import (
	"context"
	"errors"
	"testing"

	v1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/slice"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

func testScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	if err := v1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add work scheme %s ", err)
	}

	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add work scheme %s ", err)
	}

	return scheme
}

func TestGetAdditionalHeaders(t *testing.T) {
	testCases := []struct {
		name   string
		Secret *v1.Secret
		Probe  *v1alpha1.DNSHealthCheckProbe
		Verify func(t *testing.T, headers v1alpha1.AdditionalHeaders, err error)
	}{
		{
			name: "finds correct headers from secret",
			Secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe-headers",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"header": []byte("value"),
				},
				Type: "Opaque",
			},
			Probe: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe",
					Namespace: "default",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					AdditionalHeadersRef: &v1alpha1.AdditionalHeadersRef{
						Name: "probe-headers",
					},
				},
			},
			Verify: func(t *testing.T, headers v1alpha1.AdditionalHeaders, err error) {
				if err != nil {
					t.Fatalf("expected no error, got: %s", err)
				}

				if len(headers) != 1 {
					t.Fatalf("expected 1 header, got %d", len(headers))
				}

				if headers[0].Name != "header" {
					t.Fatalf("expected header name 'header' got %v", headers[0].Name)
				}

				if headers[0].Value != "value" {
					t.Fatalf("expected header value 'value' got %v", headers[0].Value)
				}
			},
		},
		{
			name: "trims whitespace from header name",
			Secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe-headers",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"  header1  ": []byte("value"),
					"  header2":   []byte("value"),
					"header3   ":  []byte("value"),
					"header4":     []byte("value"),
				},
				Type: "Opaque",
			},
			Probe: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe",
					Namespace: "default",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					AdditionalHeadersRef: &v1alpha1.AdditionalHeadersRef{
						Name: "probe-headers",
					},
				},
			},
			Verify: func(t *testing.T, headers v1alpha1.AdditionalHeaders, err error) {
				if err != nil {
					t.Fatalf("expected no error, got: %s", err)
				}

				if len(headers) != 4 {
					t.Fatalf("expected 4 header, got %d", len(headers))
				}

				expectedHeaders := v1alpha1.AdditionalHeaders{
					{
						Name: "header1", Value: "value",
					},
					{
						Name: "header2", Value: "value",
					},
					{
						Name: "header3", Value: "value",
					},
					{
						Name: "header4", Value: "value",
					},
				}

				for _, expectedHeader := range expectedHeaders {
					if !slice.Contains(headers, func(header v1alpha1.AdditionalHeader) bool {
						return header.Name == expectedHeader.Name && header.Value == expectedHeader.Value
					}) {
						t.Fatalf("expected header %+v, not present", expectedHeader)
					}
				}
			},
		},
		{
			name: "fails when header contains untrimmable whitespace",
			Secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe-headers",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"header 1": []byte("value"),
				},
				Type: "Opaque",
			},
			Probe: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe",
					Namespace: "default",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					AdditionalHeadersRef: &v1alpha1.AdditionalHeadersRef{
						Name: "probe-headers",
					},
				},
			},
			Verify: func(t *testing.T, headers v1alpha1.AdditionalHeaders, err error) {
				if !errors.Is(err, ErrInvalidHeader) {
					t.Fatalf("expected Invalid header error, got: %s", err)
				}
			},
		},
		{
			name: "error when missing secret",
			Secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe-headers",
					Namespace: "default",
				},
				Data: map[string][]byte{
					"header": []byte("value"),
				},
				Type: "Opaque",
			},
			Probe: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe",
					Namespace: "default",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					AdditionalHeadersRef: &v1alpha1.AdditionalHeadersRef{
						Name: "bad-probe-headers",
					},
				},
			},
			Verify: func(t *testing.T, headers v1alpha1.AdditionalHeaders, err error) {
				if !k8serrors.IsNotFound(err) {
					t.Fatalf("expected not found error, got: %s", err)
				}
			},
		},
		{
			name: "no error with empty secret",
			Secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe-headers",
					Namespace: "default",
				},
				Data: map[string][]byte{},
				Type: "Opaque",
			},
			Probe: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe",
					Namespace: "default",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					AdditionalHeadersRef: &v1alpha1.AdditionalHeadersRef{
						Name: "probe-headers",
					},
				},
			},
			Verify: func(t *testing.T, headers v1alpha1.AdditionalHeaders, err error) {
				if err != nil {
					t.Fatalf("expected no error, got: %s", err)
				}

				if len(headers) != 0 {
					t.Fatalf("expected 1 header, got %d", len(headers))
				}
			},
		},
		{
			name: "ignores secret in other namespace",
			Secret: &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe-headers",
					Namespace: "not-default",
				},
				Data: map[string][]byte{
					"header": []byte("value"),
				},
				Type: "Opaque",
			},
			Probe: &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "probe",
					Namespace: "default",
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					AdditionalHeadersRef: &v1alpha1.AdditionalHeadersRef{
						Name: "bad-probe-headers",
					},
				},
			},
			Verify: func(t *testing.T, headers v1alpha1.AdditionalHeaders, err error) {
				if !k8serrors.IsNotFound(err) {
					t.Fatalf("expected not found error, got: %s", err)
				}
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			f := fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(testCase.Probe, testCase.Secret).Build()
			headers, err := getAdditionalHeaders(context.Background(), f, testCase.Probe)
			testCase.Verify(t, headers, err)
		})
	}
}
