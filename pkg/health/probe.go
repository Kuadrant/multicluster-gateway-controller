package health

import (
	"context"
	"time"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Probe struct {
	ID string

	Interval  time.Duration
	Protocol  v1alpha1.HealthProtocol
	Path      string
	IPAddress string
	Host      string
	Port      int

	Notifier ProbeNotifier
	Queue    *RequestQueue

	cancel  context.CancelFunc
	started bool
	logger  logr.Logger
}

type ProbeResult struct {
	CheckedAt time.Time
	Reason    string
	Healthy   bool
}

type ProbeNotifier interface {
	Notify(ctx context.Context, result ProbeResult) error
}

func (p *Probe) Start() {
	if p.started {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.logger = log.FromContext(ctx)

	go func() {
		for {
			select {
			case <-time.After(p.Interval):
				p.Queue.EnqueueCheck(HealthRequest{
					Host:      p.Host,
					Path:      p.Path,
					Protocol:  p.Protocol,
					IPAddress: p.IPAddress,
					Port:      p.Port,
					Notifier:  p.Notifier,
				})
			case <-ctx.Done():
				return
			}
		}
	}()

	p.started = true
}

func (p *Probe) Stop() {
	if !p.started {
		return
	}

	p.logger.V(3).Info("stopping probe", "id", p.ID)
	p.cancel()
}

// func (p *Probe) Check(ctx context.Context) ProbeResult {
// 	client := http.Client{}

// 	port := 80
// 	if p.Port != 0 {
// 		port = p.Port
// 	}

// 	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://%s:%d/%s", p.IPAddress, port, p.Path), nil)
// 	if err != nil {
// 		return ProbeResult{CheckedAt: time.Now(), Healthy: false, Reason: err.Error()}
// 	}

// 	req.Header.Add("Host", p.Host)

// 	res, err := client.Do(req)
// 	if err != nil {
// 		return ProbeResult{CheckedAt: time.Now(), Healthy: false, Reason: err.Error()}
// 	}

// 	healthy := true
// 	reason := ""
// 	if res.StatusCode != 200 && res.StatusCode != 201 {
// 		healthy = false
// 		reason = fmt.Sprintf("Status code: %d", res.StatusCode)
// 	}

// 	return ProbeResult{
// 		CheckedAt: time.Now(),
// 		Healthy:   healthy,
// 		Reason:    reason,
// 	}
// }
