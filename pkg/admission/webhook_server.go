package admission

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	log "github.com/sirupsen/logrus"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	admissioningress "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/admission/ingress"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/ingress"
)

type WebhookServer struct {
	Port int

	Hosts        ingress.HostService
	Certificates ingress.CertificateService
}

func NewWebhookServer(hostService ingress.HostService, certsService ingress.CertificateService, port int) *WebhookServer {
	return &WebhookServer{
		Port: port,

		Hosts:        hostService,
		Certificates: certsService,
	}
}

func (s *WebhookServer) Start(ctx context.Context) error {
	logger := logr.New(zapr.NewLogger(zap.L()).GetSink())
	logger.Info(fmt.Sprintf("Starting webhook server at :%d", s.Port))

	mux := http.NewServeMux()

	handler, err := admissioningress.CreateHandler(s.Hosts, s.Certificates)
	if err != nil {
		log.Error("Error creating handler", err)
		return err
	}
	webhook := &webhook.Admission{
		Handler: handler,
	}

	err = webhook.InjectLogger(logger)
	if err != nil {
		return err
	}

	mux.Handle("/ingress", webhook)
	httpErr := make(chan error)
	go func() {
		httpErr <- http.ListenAndServe(fmt.Sprintf(":%d", s.Port), mux)
	}()

	select {
	case err := <-httpErr:
		return err
	case <-ctx.Done():
		ctxErr := ctx.Err()
		if errors.Is(ctxErr, context.Canceled) {
			return nil
		}

		return ctxErr
	}
}
