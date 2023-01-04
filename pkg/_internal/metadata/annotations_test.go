package metadata

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func Test_addAnnotation(t *testing.T) {
	tests := []struct {
		name            string //for name of test
		obj             metav1.Object
		annotationKey   string
		annotationValue string
		verify          func(obj metav1.Object, t *testing.T) //what we want to verify
	}{
		{ //first test starts here and...
			name: "adding an annotation when annotations are nil",
			obj: &v1.ConfigMap{ //here we set Annotations to nil
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-object",
					Annotations: nil,
				},
			}, //next we provide a key name and value
			annotationKey:   "test-key",
			annotationValue: "test-value",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 1 {
					t.Errorf("expected 1 annotation, got: %v", len(obj.GetAnnotations()))
				}
				for k, v := range obj.GetAnnotations() {
					if k != "test-key" {
						t.Errorf("expected only annotation key to be 'test-key' but found '%v'", k)
					}
					if v != "test-value" {
						t.Errorf("expected only annotation value to be 'test-value' but found '%v'", k)
					}
				}
			},
		}, //...ends here
		{
			name: "adding an annotation when annotations are empty",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-object",
					Annotations: map[string]string{}, //this is an empty map
				},
			},
			annotationKey:   "test-key",
			annotationValue: "test-value",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 1 {
					t.Errorf("expected 1 annotation, got: %v", len(obj.GetAnnotations()))
				}
				for k, v := range obj.GetAnnotations() {
					if k != "test-key" {
						t.Errorf("expected only annotation key to be 'test-key' but found '%v'", k)
					}
					if v != "test-value" {
						t.Errorf("expected only annotation value to be 'test-value' but found '%v'", k)
					}
				}
			},
		},
		{
			name: "adding an annotation when that annotation already exists",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"test-key": "different-test-value", //annotation that's stored in the map
					},
				},
			},
			annotationKey:   "test-key",
			annotationValue: "test-value",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 1 {
					t.Errorf("expected 1 annotation, got: %v", len(obj.GetAnnotations()))
				}
				for k, v := range obj.GetAnnotations() {
					if k != "test-key" {
						t.Errorf("expected only annotation key to be 'test-key' but found '%v'", k)
					}
					if v != "test-value" {
						t.Errorf("expected only annotation value to be 'test-value' but found '%v'", k)
					}
				}
			},
		},
		{
			name: "adding an annotation when that annotation already exists",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"first-key":  "test-value",
						"second-key": "test-value",
						"test-key":   "",
					},
				},
			},
			annotationKey:   "test-key",
			annotationValue: "test-value",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 3 {
					t.Errorf("expected 3 annotation, got: %v", len(obj.GetAnnotations()))
				}
				expectedAnnotations := map[string]string{
					"first-key":  "test-value",
					"second-key": "test-value",
					"test-key":   "test-value",
				}
				if !reflect.DeepEqual(obj.GetAnnotations(), expectedAnnotations) {
					t.Errorf("expected annotations '%+v' to match expectedAnnotations: '%+v'", obj.GetAnnotations(), expectedAnnotations)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddAnnotation(tt.obj, tt.annotationKey, tt.annotationValue)
			tt.verify(tt.obj, t)
		})
	}
}

func Test_removeAnnotation(t *testing.T) {

	tests := []struct {
		name          string //for name of test
		obj           metav1.Object
		annotationKey string
		verify        func(obj metav1.Object, t *testing.T) //what we want to verify
	}{
		{ //first test starts here and...
			name: "removing an annotation when annotations are nil",
			obj: &v1.ConfigMap{ //here we set Annotations to nil
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-object",
					Annotations: nil,
				},
			}, //next we provide a key name
			annotationKey: "test-key", //We are trying to remove this key, even though it doesn't exist
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 0 {
					t.Errorf("expected 0 annotation, got: %v", len(obj.GetAnnotations()))
				}
			},
		}, //...ends here
		{
			name: "removing an annotation when annotations are empty",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-object",
					Annotations: map[string]string{}, //this is an empty map
				},
			},
			annotationKey: "test-key",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 0 {
					t.Errorf("expected 0 annotation, got: %v", len(obj.GetAnnotations()))
				}
			},
		},

		{
			name: "removing an existing annotation",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"test-key": "test-value", //annotation that's stored in the map
					},
				},
			},
			annotationKey: "test-key", //this is what we are passing to the function
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 0 {
					t.Errorf("expected 0 annotation, got: %v", len(obj.GetAnnotations()))
				}
			},
		},
		{
			name: "remove an existing annotation",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"first-key":  "test-value",
						"second-key": "test-value",
						"test-key":   "",
					},
				},
			},
			annotationKey: "test-key",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 2 {
					t.Errorf("expected 2 annotations, got: %v", len(obj.GetAnnotations()))
				}
				expectedAnnotations := map[string]string{
					"first-key":  "test-value",
					"second-key": "test-value",
				}
				if !reflect.DeepEqual(obj.GetAnnotations(), expectedAnnotations) {
					t.Errorf("expected annotations '%+v' to match expectedAnnotations: '%+v'", obj.GetAnnotations(), expectedAnnotations)
				}
			},
		},
		{
			name: "remove an annotation that does not exist in the map",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"first-key":  "test-value",
						"second-key": "test-value",
						"third-key":  "",
					},
				},
			},
			annotationKey: "fourth-key",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetAnnotations()) != 3 {
					t.Errorf("expected 3 annotations, got: %v", len(obj.GetAnnotations()))
				}
				expectedAnnotations := map[string]string{
					"first-key":  "test-value",
					"second-key": "test-value",
					"third-key":  "",
				}
				if !reflect.DeepEqual(obj.GetAnnotations(), expectedAnnotations) {
					t.Errorf("expected annotations '%+v' to match expectedAnnotations: '%+v'", obj.GetAnnotations(), expectedAnnotations)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RemoveAnnotation(tt.obj, tt.annotationKey)
			tt.verify(tt.obj, t)
		})
	}
}

func Test_hasAnnotation(t *testing.T) {
	tests := []struct {
		name       string
		obj        metav1.Object
		annotation string
		expect     bool
	}{
		{
			name: "existing annotation found",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"test-key": "value",
					},
				},
			},
			annotation: "test-key",
			expect:     true,
		},
		{
			name: "existing annotation not found",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Annotations: map[string]string{
						"test-fail": "value",
					},
				},
			},
			annotation: "test-key",
			expect:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasAnnotation(tt.obj, tt.annotation)
			if !got == tt.expect {
				t.Errorf("expected '%v' got '%v'", tt.expect, got)
			}
		})
	}
}
