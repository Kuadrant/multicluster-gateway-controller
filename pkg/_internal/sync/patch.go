package sync

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer/mutator"
	"gomodules.xyz/jsonpatch/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetPatchAnnotation sets the mgc-syncer-patch annotation in obj with the
// resulting patch of applying mutation to obj
func SetPatchAnnotation[T client.Object](mutation func(T), downstreamTarget string, obj T) error {
	patch, err := PatchFor(mutation, obj)
	if err != nil {
		return err
	}
	if patch == nil {
		return nil
	}

	annotationKey := fmt.Sprintf("%s%s", mutator.JSONPatchAnnotationPrefix, downstreamTarget)
	metadata.AddAnnotation(obj, annotationKey, string(patch))

	return nil
}

func PatchForType[T client.Object](mutation func(T)) ([]byte, error) {
	template := *new(T)
	t := reflect.TypeOf(template).Elem()

	original := reflect.New(t).Interface().(T)

	return PatchFor(mutation, original)
}

func PatchFor[T client.Object](mutation func(T), original T) ([]byte, error) {
	updated := original.DeepCopyObject().(T)
	mutation(updated)

	if reflect.DeepEqual(original, updated) {
		return nil, nil
	}

	updatedJson, err := json.Marshal(updated)
	if err != nil {
		return nil, err
	}
	originalJson, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}

	patch, err := jsonpatch.CreatePatch(originalJson, updatedJson)
	if err != nil {
		return nil, err
	}

	return json.Marshal(patch)
}
