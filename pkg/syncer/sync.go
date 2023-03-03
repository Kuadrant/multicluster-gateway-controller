package syncer

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"

	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multi-cluster-traffic-controller/pkg/_internal/slice"
)

const (
	MCTC_SYNC_ANNOTATION_PREFIX   = "mctc-sync-agent/"
	MCTC_SYNC_ANNOTATION_WILDCARD = "all"
)

type Syncer interface {
	Handle(unstructured unstructured.Unstructured) error
}

type Config struct {
	ClusterID       string
	GVRs            []string
	InformerFactory dynamicinformer.DynamicSharedInformerFactory
	NeverSyncedGVRs []string
	UpstreamNS      string
	DownstreamNS    string
	Syncer          Syncer
}

type InformerEventsDecorator func(cfg Config, informer informers.GenericInformer, gvr *schema.GroupVersionResource, c SyncController) error

type SyncController interface {
	AddToQueue(schema.GroupVersionResource, interface{})
}

type SyncRunnable struct {
	cfg               Config
	informerDecorator InformerEventsDecorator
	controller        SyncController
}

func (r *SyncRunnable) Start(ctx context.Context) error {
	for _, gvrStr := range r.cfg.GVRs {
		// Some GVRs should never be synced (e.g. 'pods')
		if slice.ContainsString(r.cfg.NeverSyncedGVRs, gvrStr) {
			continue
		}
		gvr, _ := schema.ParseResourceArg(gvrStr)
		informer := r.cfg.InformerFactory.ForResource(*gvr)
		err := r.informerDecorator(r.cfg, informer, gvr, r.controller)
		if err != nil {
			return fmt.Errorf("error decorating informer for GVR events: %v", err.Error())
		}
		informer.Informer().Run(ctx.Done())
	}
	<-ctx.Done()
	return nil
}

func GetSyncerRunnable(cfg Config, informerEventDecorator InformerEventsDecorator, c SyncController) *SyncRunnable {
	return &SyncRunnable{
		cfg:               cfg,
		informerDecorator: informerEventDecorator,
		controller:        c,
	}
}

// InformerForGVR is an informer Decorator which adds generic event handlers to an informer
func InformerForGVR(cfg Config, informer informers.GenericInformer, gvr *schema.GroupVersionResource, c SyncController) error {
	_, err := informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(objInterface interface{}) {
			metaAccessor, err := meta.Accessor(objInterface)
			if err != nil {
				return
			}
			if metaAccessor.GetNamespace() != cfg.UpstreamNS {
				return
			}
			value := metadata.GetAnnotation(metaAccessor, MCTC_SYNC_ANNOTATION_PREFIX+cfg.ClusterID)
			if value != "true" {
				// no specific annotation for this cluster, is a wildcard annotation present?
				value = metadata.GetAnnotation(metaAccessor, MCTC_SYNC_ANNOTATION_PREFIX+MCTC_SYNC_ANNOTATION_WILDCARD)
				if value != "true" {
					return
				}
			}

			c.AddToQueue(*gvr, objInterface)
		},
		UpdateFunc: func(oldObjInterface, newObjInterface interface{}) {
			metaAccessor, err := meta.Accessor(newObjInterface)
			if err != nil {
				return
			}
			if metaAccessor.GetNamespace() != cfg.UpstreamNS {
				return
			}
			value := metadata.GetAnnotation(metaAccessor, MCTC_SYNC_ANNOTATION_PREFIX+cfg.ClusterID)
			if value != "true" {
				// no specific annotation for this cluster, is a wildcard annotation present?
				value = metadata.GetAnnotation(metaAccessor, MCTC_SYNC_ANNOTATION_PREFIX+MCTC_SYNC_ANNOTATION_WILDCARD)
				if value != "true" {
					return
				}
			}
			c.AddToQueue(*gvr, newObjInterface)
		},
		DeleteFunc: func(objInterface interface{}) {
			metaAccessor, err := meta.Accessor(objInterface)
			if err != nil {
				return
			}
			if metaAccessor.GetNamespace() != cfg.UpstreamNS {
				return
			}
			value := metadata.GetAnnotation(metaAccessor, MCTC_SYNC_ANNOTATION_PREFIX+cfg.ClusterID)
			if value != "true" {
				// no specific annotation for this cluster, is a wildcard annotation present?
				value = metadata.GetAnnotation(metaAccessor, MCTC_SYNC_ANNOTATION_PREFIX+MCTC_SYNC_ANNOTATION_WILDCARD)
				if value != "true" {
					return
				}
			}
			c.AddToQueue(*gvr, objInterface)
		},
	})
	return err
}
