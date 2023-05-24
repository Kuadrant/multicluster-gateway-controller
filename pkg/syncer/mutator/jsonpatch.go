package mutator

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	jsonpatch "github.com/evanphx/json-patch/v5"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/syncer"
)

const (
	JSONPatchAnnotationPrefix = "mctc-syncer-patch/"
)

type JSONPatch struct {
}

func (m *JSONPatch) GetName() string {
	return "JSON Patch"
}

func (m *JSONPatch) Mutate(cfg syncer.MutatorConfig, obj *unstructured.Unstructured) error {
	patchString := metadata.GetAnnotation(obj, JSONPatchAnnotationPrefix+cfg.ClusterID)
	if patchString == "" {
		return nil
	}

	patch, err := jsonpatch.DecodePatch([]byte(patchString))
	if err != nil {
		return fmt.Errorf("error decoding patch: %v", err.Error())
	}
	objBytes, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	objBytes, err = patch.Apply(objBytes)
	if err != nil {
		return err
	}

	err = json.Unmarshal(objBytes, obj)
	if err != nil {
		return fmt.Errorf("failed to unmarshal patched JSON to object: %v", err.Error())
	}

	return nil
}
