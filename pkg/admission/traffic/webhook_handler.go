package traffic

import (
	"bytes"
	"context"

	trafficctrl "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/traffic"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
	trafficapi "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"

	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

// TrafficWebhookHandler implements the admission Handler interface with the
// generic logic to handle requests for an object that can be wrapped around
// the traffic interface
type TrafficWebhookHandler[T runtime.Object] struct {
	// Creates a new traffic object. Example, an empty *Ingress
	NewObj func() T
	// Creates a traffic accessor for the given traffic object
	NewAccessor func(T) trafficapi.Interface

	HostService trafficctrl.HostService
	CertService trafficctrl.CertificateService

	decoder    *admission.Decoder
	serializer *json.Serializer
}

func NewTrafficWebhookHandler[T runtime.Object](
	addToScheme func(s *runtime.Scheme) error,
	newObj func() T,
	newAccessor func(T) trafficapi.Interface,

	hostService trafficctrl.HostService,
	certService trafficctrl.CertificateService,
) (*TrafficWebhookHandler[T], error) {
	scheme := runtime.NewScheme()
	if err := addToScheme(scheme); err != nil {
		return nil, err
	}

	serializer := json.NewSerializerWithOptions(
		json.DefaultMetaFactory,
		scheme, scheme,
		json.SerializerOptions{},
	)

	decoder, err := admission.NewDecoder(scheme)
	if err != nil {
		return nil, err
	}

	return &TrafficWebhookHandler[T]{
		NewObj:      newObj,
		NewAccessor: newAccessor,

		HostService: hostService,
		CertService: certService,

		serializer: serializer,
		decoder:    decoder,
	}, nil
}

func (h *TrafficWebhookHandler[T]) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.DryRun != nil && *req.DryRun {
		return admission.Allowed("skipped due to dry run")
	}

	obj := h.NewObj()
	if err := h.decoder.Decode(req, obj); err != nil {
		return admission.Errored(-1, err)
	}

	original := obj.DeepCopyObject().(T)

	allowed, err := h.handle(ctx, obj)
	if err != nil {
		return admission.Errored(-1, err)
	}

	if !allowed {
		return admission.Denied("")
	}

	if !equality.Semantic.DeepEqual(original, obj) {
		var originalSerialised bytes.Buffer
		var currentSerialised bytes.Buffer

		h.serializer.Encode(original, &originalSerialised)
		h.serializer.Encode(obj, &currentSerialised)

		return admission.PatchResponseFromRaw(
			originalSerialised.Bytes(),
			currentSerialised.Bytes(),
		)
	}

	return admission.Allowed("")
}

func (h *TrafficWebhookHandler[T]) handle(ctx context.Context, obj T) (bool, error) {
	trafficAccessor := h.NewAccessor(obj)

	// verify host is correct
	// no managed host assigned assign one
	// create empty DNSRecord with assigned host
	_, managedHostRecords, err := h.HostService.EnsureManagedHost(ctx, trafficAccessor)
	if err != nil && err != dns.AlreadyAssignedErr {
		return false, err
	}

	for _, managedHostRecord := range managedHostRecords {
		if err := trafficAccessor.AddManagedHost(managedHostRecord.Name); err != nil {
			return false, err
		}
		// create certificate resource for assigned host
		if err := h.CertService.EnsureCertificate(ctx, managedHostRecord.Name, managedHostRecord); err != nil && !k8serrors.IsAlreadyExists(err) {
			return false, err
		}
		// when certificate ready copy secret (need to add event handler for certs)
		// only once certificate is ready update DNS based status of ingress
		secret, err := h.CertService.GetCertificateSecret(ctx, managedHostRecord.Name)
		if err != nil && !k8serrors.IsNotFound(err) {
			return false, err
		}

		// If the secret was not found, `GetCertificateSecret` returns `nil`
		// we still set the TLS expecting the name to match the host name
		if secret == nil {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: managedHostRecord.Name,
				},
			}
		}

		trafficAccessor.AddTLS(managedHostRecord.Name, secret)
	}

	return true, nil
}
