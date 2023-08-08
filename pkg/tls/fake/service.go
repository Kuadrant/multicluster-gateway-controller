//go:build unit

package fake

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

type FakeCertificateService struct {
	controlClient client.Client
}

func (s *FakeCertificateService) EnsureCertificate(_ context.Context, name, host string, _ metav1.Object) error {
	if host == testutil.FailEnsureCertHost {
		return fmt.Errorf(testutil.FailEnsureCertHost)
	}
	return nil
}

func (s *FakeCertificateService) GetCertificateSecret(ctx context.Context, host string, namespace string) (*v1.Secret, error) {

	tlsSecret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      host,
		Namespace: namespace,
	}}
	if err := s.controlClient.Get(ctx, client.ObjectKeyFromObject(tlsSecret), tlsSecret); err != nil {
		return nil, err
	}
	return tlsSecret, nil
}

func NewTestCertificateService(client client.Client) *FakeCertificateService {
	return &FakeCertificateService{controlClient: client}
}
