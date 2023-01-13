package multiClusterWatch

import (
	"context"
	"fmt"
	"time"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeUtil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	trafficController "github.com/Kuadrant/multi-cluster-traffic-controller/pkg/controllers/traffic"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/dns"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/tls"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/traffic"
)

const (
	RESYNC_PERIOD = 30 * time.Minute
)

type ResourceHandlerFactory func(c *rest.Config, controlClient client.Client) (ResourceHandler, error)

type ResourceHandler interface {
	Handle(context.Context, runtime.Object) (ctrl.Result, error)
}

func NewTrafficHandlerFactory(dnsService *dns.Service, tlsService *tls.Service) ResourceHandlerFactory {
	return func(config *rest.Config, controlClient client.Client) (ResourceHandler, error) {
		c, err := client.New(config, client.Options{})
		if err != nil {
			return nil, err
		}
		trafficHandler := &trafficController.Reconciler{
			WorkloadClient: c,
			Hosts:          dnsService,
			Certificates:   tlsService,
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
	Queue       workqueue.RateLimitingInterface
	indexer     cache.Indexer
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

func (w *ClusterWatcher) WatchIngress(sharedInformer informers.SharedInformerFactory) error {

	informer := sharedInformer.Networking().V1().Ingresses().Informer()
	w.indexer = informer.GetIndexer()
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Log.Info("got add event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*networkingv1.Ingress).Namespace+"/"+obj.(*networkingv1.Ingress).Name)
			w.Enqueue(obj)
		},
		UpdateFunc: func(old, obj interface{}) {
			log.Log.Info("got update event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*networkingv1.Ingress).Namespace+"/"+obj.(*networkingv1.Ingress).Name)
			w.Enqueue(obj)
		},
		DeleteFunc: func(obj interface{}) {
			log.Log.Info("got delete event for ingress", "cluster watcher", w.ClusterName, "ingress", obj.(*networkingv1.Ingress).Namespace+"/"+obj.(*networkingv1.Ingress).Name)
			w.Enqueue(obj)
		},
	})
	if err != nil {
		return err
	}
	return nil
}

func (w *ClusterWatcher) Start(ctx context.Context) error {
	defer runtimeUtil.HandleCrash()
	defer w.Queue.ShutDown()
	informerFactory := informers.NewSharedInformerFactory(w.client, RESYNC_PERIOD)

	if err := w.WatchIngress(informerFactory); err != nil {
		return err
	}
	informerFactory.Start(ctx.Done())
	informerFactory.WaitForCacheSync(ctx.Done())

	log.Log.Info("started watcher events", "cluster watcher", w.ClusterName)
	go wait.UntilWithContext(ctx, w.startWorker, time.Second)
	<-ctx.Done()
	log.Log.Info("closing watch", "cluster", w.ClusterName)
	return nil
}

func (w *ClusterWatcher) startWorker(ctx context.Context) {
	for w.processNextWorkItem(ctx) {
	}
}

func (w *ClusterWatcher) Enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		runtimeUtil.HandleError(err)
		return
	}
	w.Queue.Add(key)
}

func (w *ClusterWatcher) EnqueueAfter(obj interface{}, dur time.Duration) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtimeUtil.HandleError(err)
		return
	}
	w.Queue.AddAfter(key, dur)
}

func (w *ClusterWatcher) process(ctx context.Context, key string) error {
	object, exists, err := w.indexer.GetByKey(key)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !exists {
		// The Ingress has been deleted, so we remove any Ingress to Service tracking.
		return nil
	}

	currentState := object.(*networkingv1.Ingress)
	targetState := currentState.DeepCopy()
	targetStateReadWriter := traffic.NewIngress(targetState)
	res, err := w.Handler.Handle(ctx, targetStateReadWriter)
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepEqual(currentState, targetState) {
		//write back to cluster
		if _, err := w.client.NetworkingV1().Ingresses(targetState.Namespace).Update(ctx, targetState, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	if res.Requeue {
		log.Log.V(10).Info("requeuing object after ", "duration", res.RequeueAfter)
		w.EnqueueAfter(currentState, res.RequeueAfter)
	}
	return nil
}

func (w *ClusterWatcher) processNextWorkItem(ctx context.Context) bool {
	// Wait until there is a new item in the working queue
	k, quit := w.Queue.Get()
	if quit {
		return false
	}
	key := k.(string)

	// No matter what, tell the queue we're done with this key,
	// to unblock other workers.
	defer w.Queue.Done(key)

	err := w.process(ctx, key)

	// Reconcile worked, nothing else to do for this work-queue item
	if err == nil {
		w.Queue.Forget(key)
		return true
	}
	// Re-enqueue up to 5 times
	n := w.Queue.NumRequeues(key)
	if n < 5 {
		log.Log.Error(err, "Re-queuing after reconciliation error", "key", key, "retries", n)
		w.Queue.AddRateLimited(key)
		return true
	}

	// Give up and report error elsewhere.
	w.Queue.Forget(key)
	runtimeUtil.HandleError(err)
	log.Log.Error(err, "Dropping key after max failed retries", "key", key, "retries", n)

	return true
}

func NewClusterWatcher(mgr manager.Manager, config *rest.Config, handlerFactory ResourceHandlerFactory) (Watcher, error) {
	controllerName := fmt.Sprintf("%s/%s", config.ServerName, "ingress")
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)
	log.Log.Info("creating new cluster watcher", "host", config.Host)
	watcherClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	handler, err := handlerFactory(config, mgr.GetClient())
	if err != nil {
		return nil, err
	}
	watcher := &ClusterWatcher{client: watcherClient, ClusterName: config.Host, Handler: handler, Queue: queue}
	err = mgr.Add(watcher)
	if err != nil {
		log.Log.Error(err, "error Adding cluster watcher the Manager")
	}

	return watcher, nil
}
