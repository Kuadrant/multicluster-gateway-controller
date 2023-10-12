package policysync

import (
	"context"

	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type PolicyInformersManager struct {
	manager manager.Manager

	InformerFactory dynamicinformer.DynamicSharedInformerFactory
}

func NewPolicyInformersManager(informerFactory dynamicinformer.DynamicSharedInformerFactory) *PolicyInformersManager {
	return &PolicyInformersManager{
		InformerFactory: informerFactory,
	}
}

func (p *PolicyInformersManager) SetupWithManager(mgr manager.Manager) error {
	p.manager = mgr
	return p.manager.Add(p)
}

func (p *PolicyInformersManager) Start(ctx context.Context) error {
	done := make(chan struct{})

	p.InformerFactory.Start(done)
	p.InformerFactory.WaitForCacheSync(done)

	err := <-ctx.Done()
	done <- err

	return nil
}

func (p *PolicyInformersManager) AddInformer(informer cache.SharedIndexInformer) error {
	return p.manager.Add(&InformerRunnable{Informer: informer})
}

type InformerRunnable struct {
	Informer cache.SharedIndexInformer
}

func (r *InformerRunnable) Start(ctx context.Context) error {
	r.Informer.Run(ctx.Done())
	return nil
}
