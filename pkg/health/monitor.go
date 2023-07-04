package health

import (
	"context"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type Monitor struct {
	Probes []*Probe

	mux sync.Mutex
}

func NewMonitor() *Monitor {
	return &Monitor{
		Probes: make([]*Probe, 0),
	}
}

func (m *Monitor) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	<-ctx.Done()
	m.mux.Lock()
	defer m.mux.Unlock()

	logger.Info("Stopping health check monitor")

	for _, probe := range m.Probes {
		probe.Stop()
	}

	return nil
}

var _ manager.Runnable = &Monitor{}

func (m *Monitor) HasProbe(id string) bool {
	m.mux.Lock()
	defer m.mux.Unlock()

	for _, probe := range m.Probes {
		if probe.ID == id {
			return true
		}
	}

	return false
}

func (m *Monitor) UpdateProbe(id string, update func(*Probe)) {
	m.mux.Lock()
	defer m.mux.Unlock()

	for _, probe := range m.Probes {
		if probe.ID == id {
			update(probe)
		}
	}
}

func (m *Monitor) AddProbe(probe *Probe) bool {
	m.mux.Lock()
	defer m.mux.Unlock()

	for _, existingProbe := range m.Probes {
		if probe.ID == existingProbe.ID {
			return false
		}
	}

	m.Probes = append(m.Probes, probe)
	probe.Start()
	return true
}

func (m *Monitor) RemoveProbe(id string) {
	m.mux.Lock()
	defer m.mux.Unlock()

	updatedProbes := []*Probe{}

	for _, probe := range m.Probes {
		if probe.ID == id {
			probe.Stop()
		} else {
			updatedProbes = append(updatedProbes, probe)
		}
	}

	m.Probes = updatedProbes
}
