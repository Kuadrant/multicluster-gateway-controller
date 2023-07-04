package health

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RequestQueue funnels incoming health check requests from health probes,
// processing them one at a time and spacing them by a specified duration
type RequestQueue struct {
	Throttle time.Duration

	requests []HealthRequest
	logger   logr.Logger

	mux sync.Mutex
}

func NewRequestQueue(throttle time.Duration) *RequestQueue {
	return &RequestQueue{
		Throttle: throttle,
		requests: make([]HealthRequest, 0),
	}
}

type HealthRequest struct {
	Host, Path, IPAddress string
	Protocol              v1alpha1.HealthProtocol
	Port                  int

	Notifier ProbeNotifier
}

func (q *RequestQueue) EnqueueCheck(req HealthRequest) {
	q.mux.Lock()
	defer q.mux.Unlock()

	q.requests = append(q.requests, req)
}

// deqeue takes the next element of the queue and returns it. It blocks
// if the queue is empty, and returns false if the context is cancelled
func (q *RequestQueue) dequeue(ctx context.Context) (HealthRequest, bool) {
	reqChn := make(chan HealthRequest)

	go func() {
		for {
			select {
			case <-ctx.Done():
				close(reqChn)
				return
			default:
			}

			q.mux.Lock()
			if len(q.requests) == 0 {
				q.mux.Unlock()
				runtime.Gosched()
				continue
			}

			req := q.requests[0]
			q.requests = q.requests[1:]
			q.mux.Unlock()

			reqChn <- req
		}

	}()

	req, ok := <-reqChn
	return req, ok
}

func (q *RequestQueue) Start(ctx context.Context) error {
	q.logger = log.FromContext(ctx)
	defer q.logger.Info("Stopping health check queue")

	for {
		select {
		case <-ctx.Done():
			if ctx.Err() != context.Canceled {
				return ctx.Err()
			}
			return nil
		case <-time.After(q.Throttle):
			req, ok := q.dequeue(ctx)
			if !ok {
				return nil
			}

			q.process(ctx, req)
		}
	}
}

func (q *RequestQueue) process(ctx context.Context, req HealthRequest) {
	go func() {
		result := q.performRequest(ctx, req)
		if err := req.Notifier.Notify(ctx, result); err != nil {
			q.logger.Error(err, "failed to notify health check result")
		}
	}()
}

func (q *RequestQueue) performRequest(ctx context.Context, req HealthRequest) ProbeResult {
	q.logger.V(3).Info("performing health check", "request", req)

	client := http.Client{}

	// Default port to 80
	port := 80
	if req.Port != 0 {
		port = req.Port
	}

	// Build the http request
	httpReq, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s://%s:%d/%s", req.Protocol.ToScheme(), req.IPAddress, port, req.Path), nil)
	if err != nil {
		return ProbeResult{CheckedAt: time.Now(), Healthy: false, Reason: err.Error()}
	}

	// Set the Host header
	httpReq.Header.Add("Host", req.Host)

	// Send the request
	res, err := client.Do(httpReq)
	if err != nil {
		return ProbeResult{CheckedAt: time.Now(), Healthy: false, Reason: err.Error()}
	}

	// Create the result based on the response
	healthy := true
	reason := ""
	if res.StatusCode != 200 && res.StatusCode != 201 {
		healthy = false
		reason = fmt.Sprintf("Status code: %d", res.StatusCode)
	}

	return ProbeResult{
		CheckedAt: time.Now(),
		Healthy:   healthy,
		Reason:    reason,
	}
}
