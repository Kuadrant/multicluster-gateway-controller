package dns

import (
	"context"
	"errors"
	"fmt"
	gonet "net"
	"sync"
	"time"

	"github.com/miekg/dns"
)

var (
	NoSuchHost = errors.New("no such host")
)

func IsNoSuchHostError(err error) bool {
	return err.Error() == NoSuchHost.Error()
}

type HostResolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]HostAddress, error)
}

type HostAddress struct {
	Host string
	IP   gonet.IP
	TTL  time.Duration
	TXT  string
}

type DefaultHostResolver struct {
	Client dns.Client
}

func NewDefaultHostResolver() *DefaultHostResolver {
	return &DefaultHostResolver{
		Client: dns.Client{},
	}
}

func (hr *DefaultHostResolver) LookupIPAddr(ctx context.Context, host string) ([]HostAddress, error) {
	cfg, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return nil, err
	}

	for _, server := range cfg.Servers {
		m := dns.Msg{}
		m.SetQuestion(fmt.Sprintf("%s.", host), dns.TypeA)

		r, _, err := hr.Client.ExchangeContext(ctx, &m, fmt.Sprintf("%s:53", server))
		if err != nil {
			return nil, err
		}

		if len(r.Answer) == 0 {
			continue
		}

		results := make([]HostAddress, 0, len(r.Answer))
		for _, answer := range r.Answer {
			if a, ok := answer.(*dns.A); ok {
				results = append(results, HostAddress{
					Host: host,
					IP:   a.A,
					TTL:  time.Duration(a.Hdr.Ttl) * time.Second,
				})
			}
		}

		return results, nil
	}

	return nil, errors.New("no records found for host")
}

type SafeHostResolver struct {
	HostResolver

	mu sync.Mutex
}

func NewSafeHostResolver(inner HostResolver) *SafeHostResolver {
	return &SafeHostResolver{
		HostResolver: inner,
	}
}

func (r *SafeHostResolver) LookupIPAddr(ctx context.Context, host string) ([]HostAddress, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.HostResolver.LookupIPAddr(ctx, host)
}
