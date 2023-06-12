//go:build unit

package mutator

import (
	"testing"

	"github.com/go-logr/logr/testr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	gatewayapi "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer/status"
)

func Test_AnnotationCleaner(t *testing.T) {
	unit := &AnnotationCleaner{}
	config := syncer.MutatorConfig{
		ClusterID: "test_cluster_id",
		Logger:    testr.New(t),
	}
	scenarios := []struct {
		name   string //for name of test
		obj    interface{}
		verify func(obj *unstructured.Unstructured, err error, t *testing.T) //what we want to verify
	}{
		{
			name: "all control plane annotations are removed",
			obj: &gatewayapi.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						JSONPatchAnnotationPrefix + config.ClusterID:                  "test",
						syncer.MGC_SYNC_ANNOTATION_PREFIX + config.ClusterID:          "test",
						status.SyncerClusterStatusAnnotationPrefix + config.ClusterID: "test",
						"kubectl.kubernetes.io/last-applied-configuration":            "test",
					},
				},
			},
			verify: func(obj *unstructured.Unstructured, err error, t *testing.T) {
				if err != nil {
					t.Fatalf("expected error to be nil, got '%v'", err.Error())
				}
				verifyNoMctcAnnotations(obj, config, t)
			},
		},
		{
			name: "none control plane annotations are preserved",
			obj: &gatewayapi.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"phils-mad-test": "test",
					},
				},
			},
			verify: func(obj *unstructured.Unstructured, err error, t *testing.T) {
				if err != nil {
					t.Fatalf("expected error to be nil, got '%v'", err.Error())
				}
				verifyNoMctcAnnotations(obj, config, t)
				if !metadata.HasAnnotation(obj, "phils-mad-test") {
					t.Fatalf("Expected annotation '%v' is missing", "phils-mad-test")
				}
			},
		},
		{
			name: "no errors if some annotations present and some missing",
			obj: &gatewayapi.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						JSONPatchAnnotationPrefix + config.ClusterID:       "test",
						"kubectl.kubernetes.io/last-applied-configuration": "test",
					},
				},
			},
			verify: func(obj *unstructured.Unstructured, err error, t *testing.T) {
				if err != nil {
					t.Fatalf("expected error to be nil, got '%v'", err.Error())
				}
				verifyNoMctcAnnotations(obj, config, t)
			},
		},
	}
	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			unstrObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(scenario.obj)
			unstr := &unstructured.Unstructured{
				Object: unstrObj,
			}
			if err != nil {
				t.Fatalf("unexpected error creating unstructured object: '%v'", err)
			}

			err = unit.Mutate(config, unstr)
			scenario.verify(unstr, err, t)
		})
	}
}

func verifyNoMctcAnnotations(obj *unstructured.Unstructured, config syncer.MutatorConfig, t *testing.T) {
	if metadata.HasAnnotation(obj, JSONPatchAnnotationPrefix+config.ClusterID) {
		t.Fatalf("found annotation '%v' expected absent", JSONPatchAnnotationPrefix+config.ClusterID)
	}
	if metadata.HasAnnotation(obj, syncer.MGC_SYNC_ANNOTATION_PREFIX+config.ClusterID) {
		t.Fatalf("found annotation '%v' expected absent", syncer.MGC_SYNC_ANNOTATION_PREFIX+config.ClusterID)
	}
	if metadata.HasAnnotation(obj, status.SyncerClusterStatusAnnotationPrefix+config.ClusterID) {
		t.Fatalf("found annotation '%v' expected absent", status.SyncerClusterStatusAnnotationPrefix+config.ClusterID)
	}
	if metadata.HasAnnotation(obj, "kubectl.kubernetes.io/last-applied-configuration") {
		t.Fatalf("found annotation '%v' expected absent", "kubectl.kubernetes.io/last-applied-configuration")
	}
}
