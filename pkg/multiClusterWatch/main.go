package multiClusterWatch

import (
	"context"
	"net/url"
	"strings"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	trafficController "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/traffic"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
)

const (
	RESYNC_PERIOD = 30 * time.Minute
)

type ResourceHandlerFactory func(host string, workloadClient client.Client, controlClient client.Client) (ResourceHandler, error)

type ResourceHandler interface {
	Handle(context.Context, runtime.Object) (ctrl.Result, error)
	SetWorkloadClient(client.Client)
	SetControlPlaneClient(client.Client)
}

func NewTrafficHandlerFactory() ResourceHandlerFactory {
	return func(host string, workloadClient client.Client, controlClient client.Client) (ResourceHandler, error) {
		hostUrl, err := url.ParseRequestURI(host)
		if err != nil {
			return nil, err
		}
		trafficHandler := &trafficController.Reconciler{
			WorkloadClient: workloadClient,
			ControlClient:  controlClient,
			ClusterID:      strings.Replace(hostUrl.Host, ":", "_", 1),
		}
		return trafficHandler, nil
	}
}

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
	HandlerFactory  ResourceHandlerFactory
}

type ClusterWatcher struct {
	ClusterName string
	client      kubernetes.Interface
	Handler     ResourceHandler
}

func (w *WatchController) WatchCluster(config *rest.Config) (Watcher, error) {
	if w.watchers == nil {
		w.watchers = map[string]Watcher{}
	}

	if w.watchers[config.Host] != nil {
		return w.watchers[config.Host], nil
	}

	watcher, err := NewClusterWatcher(w.Manager, config, w.HandlerFactory)
	if err != nil {
		return nil, err
	}

	w.watchers[config.Host] = watcher
	return watcher, nil
}

func (w *ClusterWatcher) Start(ctx context.Context) error {
	log.Log.Info("Starting cluster watcher", "name", w.ClusterName)

	informerFactory := informers.NewSharedInformerFactory(w.client, RESYNC_PERIOD)

	informer := informerFactory.Networking().V1().Ingresses().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Log.Info("got add event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*networkingv1.Ingress).Namespace+"/"+obj.(*networkingv1.Ingress).Name)
			current := obj.(*networkingv1.Ingress)
			target := current.DeepCopy()
			targetAccessor := traffic.NewIngress(target)
			_, _ = w.Handler.Handle(ctx, targetAccessor)
			//todo handle requeue and errors
			if !equality.Semantic.DeepEqual(current, target) {
				//write back to cluster
				_, _ = w.client.NetworkingV1().Ingresses(target.Namespace).Update(ctx, target, metav1.UpdateOptions{})
			}
		},
		UpdateFunc: func(old, obj interface{}) {
			log.Log.Info("got update event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*networkingv1.Ingress).Namespace+"/"+obj.(*networkingv1.Ingress).Name)
			current := obj.(*networkingv1.Ingress)
			target := current.DeepCopy()
			targetAccessor := traffic.NewIngress(target)
			_, _ = w.Handler.Handle(ctx, targetAccessor)
			//todo handle requeue and errors
			if !equality.Semantic.DeepEqual(current, target) {
				//write back to cluster
				_, _ = w.client.NetworkingV1().Ingresses(target.Namespace).Update(ctx, target, metav1.UpdateOptions{})
			}
		},
		DeleteFunc: func(obj interface{}) {
			log.Log.Info("got delete event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*networkingv1.Ingress).Namespace+"/"+obj.(*networkingv1.Ingress).Name)
			current := obj.(*networkingv1.Ingress)
			target := current.DeepCopy()
			targetAccessor := traffic.NewIngress(target)
			_, _ = w.Handler.Handle(ctx, targetAccessor)
			//todo handle requeue and errors
			if !equality.Semantic.DeepEqual(current, target) {
				//write back to cluster
				_, _ = w.client.NetworkingV1().Ingresses(target.Namespace).Update(ctx, target, metav1.UpdateOptions{})
			}
		},
	})

	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	log.Log.Info("started watcher events", "cluster watcher", w.ClusterName)

	<-ctx.Done()
	log.Log.Info("closing watch", "cluster", w.ClusterName)
	return nil
}

func NewClusterWatcher(mgr manager.Manager, config *rest.Config, handlerFactory ResourceHandlerFactory) (Watcher, error) {
	log.Log.Info("creating new cluster watcher", "host", config.Host)
	watcherClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	workloadClient, err := client.New(config, client.Options{})
	if err != nil {
		return nil, err
	}
	handler, _ := handlerFactory(config.Host, workloadClient, mgr.GetClient())
	watcher := &ClusterWatcher{client: watcherClient, ClusterName: config.Host, Handler: handler}
	err = mgr.Add(watcher)
	if err != nil {
		log.Log.Error(err, "error Adding cluster watcher the Manager")
	}

	return watcher, nil
}
