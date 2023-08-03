package tlspolicy

import (
	"context"
	"fmt"
	"reflect"

	certmanv1 "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/kuadrant/kuadrant-operator/pkg/reconcilers"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
)

func (r *TLSPolicyReconciler) reconcileCertificates(ctx context.Context, tlsPolicy *v1alpha1.TLSPolicy, gwDiffObj *reconcilers.GatewayDiff) error {
	log := crlog.FromContext(ctx)

	for _, gw := range gwDiffObj.GatewaysWithInvalidPolicyRef {
		log.V(1).Info("reconcileCertificates: gateway with invalid policy ref", "key", gw.Key())
		err := r.deleteGatewayCertificates(ctx, gw.Gateway, tlsPolicy)
		if err != nil {
			return err
		}
	}

	// Reconcile Certificates for each gateway directly referred by the policy (existing and new)
	for _, gw := range append(gwDiffObj.GatewaysWithValidPolicyRef, gwDiffObj.GatewaysMissingPolicyRef...) {
		log.V(1).Info("reconcileCertificates: gateway with valid and missing policy ref", "key", gw.Key())
		err := r.reconcileGatewayCertificates(ctx, gw.Gateway, tlsPolicy)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *TLSPolicyReconciler) reconcileGatewayCertificates(ctx context.Context, gateway *gatewayv1beta1.Gateway, tlsPolicy *v1alpha1.TLSPolicy) error {
	log := crlog.FromContext(ctx)

	log.V(1).Info("reconcileGatewayCertificates", "tlsPolicy", tlsPolicy)

	tlsHosts := make(map[corev1.ObjectReference][]string)

	for i, l := range gateway.Spec.Listeners {
		err := validateGatewayListenerBlock(field.NewPath("spec", "listeners").Index(i), l, gateway).ToAggregate()
		if err != nil {
			log.Info("Skipped a listener block: " + err.Error())
			continue
		}

		for _, certRef := range l.TLS.CertificateRefs {
			secretRef := corev1.ObjectReference{
				Name: string(certRef.Name),
			}
			if certRef.Namespace != nil {
				secretRef.Namespace = string(*certRef.Namespace)
			} else {
				secretRef.Namespace = gateway.GetNamespace()
			}
			// Gateway API hostname explicitly disallows IP addresses, so this
			// should be OK.
			tlsHosts[secretRef] = append(tlsHosts[secretRef], string(*l.Hostname))
		}
	}

	for secretRef, hosts := range tlsHosts {
		cert := r.buildCertManagerCertificate(gateway, tlsPolicy, secretRef, hosts)
		err := r.ReconcileResource(ctx, &certmanv1.Certificate{}, cert, alwaysUpdateCertificate)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			log.Error(err, "failed to reconcile Certificate resource")
			return err
		}

	}

	return nil
}

func (r *TLSPolicyReconciler) deleteGatewayCertificates(ctx context.Context, gateway *gatewayv1beta1.Gateway, tlsPolicy *v1alpha1.TLSPolicy) error {
	log := crlog.FromContext(ctx)

	listOptions := &client.ListOptions{LabelSelector: labels.SelectorFromSet(tlsCertificateLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(tlsPolicy)))}
	certList := &certmanv1.CertificateList{}
	if err := r.Client().List(ctx, certList, listOptions); err != nil {
		return err
	}

	for _, cert := range certList.Items {
		if err := r.DeleteResource(ctx, &cert); client.IgnoreNotFound(err) != nil {
			log.Error(err, "failed to delete TLS Certificate")
			return err
		}
	}

	return nil
}

func tlsCertificateLabels(gwKey, apKey client.ObjectKey) map[string]string {
	return map[string]string{
		TLSPolicyBackRefAnnotation:                              apKey.Name,
		fmt.Sprintf("%s-namespace", TLSPolicyBackRefAnnotation): apKey.Namespace,
		"gateway-namespace":                                     gwKey.Namespace,
		"gateway":                                               gwKey.Name,
	}
}

func (r *TLSPolicyReconciler) buildCertManagerCertificate(gateway *gatewayv1beta1.Gateway, tlsPolicy *v1alpha1.TLSPolicy, secretRef corev1.ObjectReference, hosts []string) *certmanv1.Certificate {
	tlsCertLabels := tlsCertificateLabels(client.ObjectKeyFromObject(gateway), client.ObjectKeyFromObject(tlsPolicy))

	crt := &certmanv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretRef.Name,
			Namespace: secretRef.Namespace,
			Labels:    tlsCertLabels,
		},
		Spec: certmanv1.CertificateSpec{
			DNSNames:   hosts,
			SecretName: secretRef.Name,
			SecretTemplate: &certmanv1.CertificateSecretTemplate{
				Labels: tlsCertLabels,
			},
			IssuerRef: tlsPolicy.Spec.IssuerRef,
			Usages:    certmanv1.DefaultKeyUsages(),
		},
	}
	translatePolicy(crt, tlsPolicy.Spec)
	return crt
}

func alwaysUpdateCertificate(existingObj, desiredObj client.Object) (bool, error) {
	existing, ok := existingObj.(*certmanv1.Certificate)
	if !ok {
		return false, fmt.Errorf("%T is not a *certmanv1.Certificate", existingObj)
	}
	desired, ok := desiredObj.(*certmanv1.Certificate)
	if !ok {
		return false, fmt.Errorf("%T is not an *certmanv1.Certificate", desiredObj)
	}

	if reflect.DeepEqual(existing.Spec, desired.Spec) {
		return false, nil
	}
	existing.Spec = desired.Spec

	return true, nil
}
