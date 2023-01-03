package multiClusterWatch

import (
	"context"
	"time"

	v1 "k8s.io/api/networking/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	RESYNC_PERIOD = 5 * time.Second
)

type Interface interface {
	WatchCluster(config *rest.Config) (Watcher, error)
}

type Watcher interface {
	Start(context.Context) error
}

type WatchController struct {
	watchers        map[string]Watcher
	InformerContext context.Context
	Manager         manager.Manager
}

type ClusterWatcher struct {
	ClusterName string
	client      kubernetes.Interface
}

func (w *WatchController) WatchCluster(config *rest.Config) (Watcher, error) {
	if w.watchers == nil {
		w.watchers = map[string]Watcher{}
	}

	if w.watchers[config.Host] != nil {
		return w.watchers[config.Host], nil
	}

	watcher, err := NewClusterWatcher(w.Manager, config)
	if err != nil {
		return nil, err
	}

	w.watchers[config.Host] = watcher
	return watcher, nil
}

func (w *ClusterWatcher) Start(ctx context.Context) error {
	log.Log.Info("Starting cluster watcher", "name", w.ClusterName)

	ingresses, err := w.client.NetworkingV1().Ingresses("").List(ctx, v12.ListOptions{})
	if err != nil {
		return err
	}

	for _, i := range ingresses.Items {
		log.Log.Info("New cluster client, can see ingress", "name", i.Namespace+"/"+i.Name, "host", w.ClusterName)
	}

	informerFactory := informers.NewSharedInformerFactory(w.client, RESYNC_PERIOD)

	informer := informerFactory.Networking().V1().Ingresses().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Log.Info("got add event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*v1.Ingress).Namespace+"/"+obj.(*v1.Ingress).Name)
		},
		UpdateFunc: func(old, obj interface{}) {
			log.Log.Info("got update event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*v1.Ingress).Namespace+"/"+obj.(*v1.Ingress).Name)
		},
		DeleteFunc: func(obj interface{}) {
			log.Log.Info("got delete event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*v1.Ingress).Namespace+"/"+obj.(*v1.Ingress).Name)
		},
	})

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	log.Log.Info("started watcher events", "cluster watcher", w.ClusterName)

	<-ctx.Done()
	log.Log.Info("closing watch", "cluster", w.ClusterName)
	return nil
}

func NewClusterWatcher(mgr manager.Manager, config *rest.Config) (Watcher, error) {
	log.Log.Info("creating new cluster watcher", "host", config.Host)
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	watcher := &ClusterWatcher{client: client, ClusterName: config.Host}
	err = mgr.Add(watcher)
	if err != nil {
		log.Log.Error(err, "error Adding cluster watcher the Manager")
	}

	return watcher, nil
}
