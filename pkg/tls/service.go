package tls

import (
	"context"
	"fmt"
	v12 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"reflect"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"
	"strings"
	"time"

	certman "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	TlsIssuerAnnotation = "kuadrant.dev/tls-issuer"
	certFinalizer       = "kuadrant.dev/certificates-cleanup"
)

type Service struct {
	controlClient client.Client
	// this is temporary setting the tenant ns in the control plane.
	// will be removed when we have auth that can map to a given ctrl plane ns
	defaultCtrlNS string
	defaultIssuer string
}

func NewService(controlClient client.Client, defaultCtrlNS, defaultIssuer string) *Service {
	return &Service{controlClient: controlClient, defaultCtrlNS: defaultCtrlNS, defaultIssuer: defaultIssuer}
}

func (s *Service) EnsureCertificate(ctx context.Context, host string, owner metav1.Object) error {
	cert := s.certificate(host, s.defaultIssuer, owner.GetNamespace())
	if err := controllerutil.SetOwnerReference(owner, cert, scheme.Scheme); err != nil {
		return err
	}
	if err := s.controlClient.Create(ctx, cert, &client.CreateOptions{}); err != nil {
		return err
	}
	return nil
}

func (s *Service) GetCertificateSecret(ctx context.Context, host string, namespace string) (*v1.Secret, error) {
	//the secret is expected to be named after the host
	tlsSecret := &v1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name:      host,
		Namespace: namespace,
	}}
	if err := s.controlClient.Get(ctx, client.ObjectKeyFromObject(tlsSecret), tlsSecret); err != nil {
		return nil, err
	}
	return tlsSecret, nil
}

func (s *Service) CleanupCertificates(ctx context.Context, owner interface{}) error {
	gateway, ok := owner.(*gatewayv1beta1.Gateway)
	if ok {
		var hostsToRemove []string
		// get names of hosts for traffic object being deleted
		for _, listener := range gateway.Spec.Listeners {
			if !strings.Contains(string(*listener.Hostname), "*.") {
				hostsToRemove = append(hostsToRemove, string(*listener.Hostname))
			}
		}
		for _, host := range hostsToRemove {
			secret := &v1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      host,
					Namespace: gateway.Namespace,
				},
			}
			err := s.controlClient.Delete(ctx, secret)
			if err != nil && !k8serrors.IsNotFound(err) {
				return fmt.Errorf("error deleting cert secret: %s", err)
			}
		}
		return nil
	}
	// TODO ingress case
	_, ok = owner.(*v12.Ingress)
	if ok {
		return nil
	}
	return fmt.Errorf("uknkown object type for a certificate deletion: %s", reflect.TypeOf(owner))
}

func (s *Service) certificate(host, issuer, controlNS string) *certman.Certificate {
	// this will be created in the control plane
	annotations := map[string]string{TlsIssuerAnnotation: issuer}
	labels := map[string]string{}
	return &certman.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:        host,
			Namespace:   controlNS,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: certman.CertificateSpec{
			SecretName: host,
			SecretTemplate: &certman.CertificateSecretTemplate{
				Labels:      labels,
				Annotations: annotations,
			},
			// TODO Some of the below should be pulled out into a CRD
			Duration: &metav1.Duration{
				Duration: time.Hour * 24 * 90, // cert lasts for 90 days
			},
			RenewBefore: &metav1.Duration{
				Duration: time.Hour * 24 * 15, // cert is renewed 15 days before hand
			},
			PrivateKey: &certman.CertificatePrivateKey{
				Algorithm: certman.RSAKeyAlgorithm,
				Encoding:  certman.PKCS1,
				Size:      2048,
			},
			Usages:   certman.DefaultKeyUsages(),
			DNSNames: []string{host},
			IssuerRef: cmmeta.ObjectReference{
				Group: "cert-manager.io",
				Kind:  "ClusterIssuer",
				Name:  issuer,
			},
		},
	}
}
