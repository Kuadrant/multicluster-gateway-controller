package metadata

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
	"testing"
)

func Test_addFinalizer(t *testing.T) {
	tests := []struct {
		name      string //for name of test
		obj       metav1.Object
		finalizer string
		verify    func(obj metav1.Object, t *testing.T) //what we want to verify
	}{
		{
			name: "adding a finalizer when finalizers are nil",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-object",
					Finalizers: nil,
				},
			},
			finalizer: "test-finalizer",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 1 {
					t.Errorf("expected 1 finalizer, got: %v", len(obj.GetFinalizers()))
				}
				for _, v := range obj.GetFinalizers() {
					if v != "test-finalizer" {
						t.Errorf("expected only finalizer value to be 'test-finalizer' but found '%v'", v)
					}
				}
			},
		},
		{
			name: "adding a finalizer when finalizers are empty",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-object",
					Finalizers: []string{}, //this is an empty map
				},
			},
			finalizer: "test-finalizer",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 1 {
					t.Errorf("expected 1 finalizer, got: %v", len(obj.GetFinalizers()))
				}
				for _, v := range obj.GetFinalizers() {
					if v != "test-finalizer" {
						t.Errorf("expected only finalizer value to be 'test-finalizer' but found '%v'", v)
					}
				}
			},
		},
		{
			name: "adding a finalizer when that finalizer already exists",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Finalizers: []string{
						"test-finalizer", //finalizer that's stored in the map
					},
				},
			},
			finalizer: "test-finalizer",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 1 {
					t.Errorf("expected 1 finalizer, got: %v", len(obj.GetFinalizers()))
				}
				for _, v := range obj.GetFinalizers() {
					if v != "test-finalizer" {
						t.Errorf("expected only finalizer value to be 'test-finalizer' but found '%v'", v)
					}
				}
			},
		},
		{
			name: "adding a finalizer when that finalizer already exists",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Finalizers: []string{
						"test-finalizer",
						"second-test-finalizer",
						"third-test-finalizer",
					},
				},
			},
			finalizer: "test-finalizer",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 3 {
					t.Errorf("expected 3 finalizer, got: %v", len(obj.GetFinalizers()))
				}
				expectedFinalizers := []string{
					"test-finalizer",
					"second-test-finalizer",
					"third-test-finalizer",
				}
				if !reflect.DeepEqual(obj.GetFinalizers(), expectedFinalizers) {
					t.Errorf("expected finalizers '%+v' to match expectedFinalizers: '%+v'", obj.GetFinalizers(), expectedFinalizers)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddFinalizer(tt.obj, tt.finalizer)
			tt.verify(tt.obj, t)
		})
	}
}

func Test_removeFinalizer(t *testing.T) {

	tests := []struct {
		name      string
		obj       metav1.Object
		finalizer string
		verify    func(obj metav1.Object, t *testing.T)
	}{
		{
			name: "removing a finalizer when finalizers are nil",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-object",
					Finalizers: nil,
				},
			},
			finalizer: "test-finalizer", //We are trying to remove this key, even though it doesn't exist
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 0 {
					t.Errorf("expected 0 finalizer, got: %v", len(obj.GetFinalizers()))
				}
			},
		},
		{
			name: "removing a finalizer when finalizers are empty",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-object",
					Finalizers: []string{}, //this is an empty map
				},
			},
			finalizer: "test-finalizer",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 0 {
					t.Errorf("expected 0 finalizer, got: %v", len(obj.GetFinalizers()))
				}
			},
		},

		{
			name: "removing an existing finalizer",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Finalizers: []string{
						"test-finalizer", //finalizer that's stored in the map
					},
				},
			},
			finalizer: "test-finalizer", //this is what we are passing to the function
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 0 {
					t.Errorf("expected 0 finalizer, got: %v", len(obj.GetFinalizers()))
				}
			},
		},
		{
			name: "remove an existing finalizer",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Finalizers: []string{
						"first-test-finalizer",
						"second-test-finalizer",
						"test-finalizer",
					},
				},
			},
			finalizer: "test-finalizer",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 2 {
					t.Errorf("expected 2 finalizers, got: %v", len(obj.GetFinalizers()))
				}
				expectedFinalizers := []string{
					"first-test-finalizer",
					"second-test-finalizer",
				}
				if !reflect.DeepEqual(obj.GetFinalizers(), expectedFinalizers) {
					t.Errorf("expected finalizers '%+v' to match expectedFinalizers: '%+v'", obj.GetFinalizers(), expectedFinalizers)
				}
			},
		},
		{
			name: "remove a finalizer that does not exist in the map",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Finalizers: []string{
						"first-test-finalizer",
						"second-test-finalizer",
						"third-test-finalizer",
					},
				},
			},
			finalizer: "fourth-key",
			verify: func(obj metav1.Object, t *testing.T) {
				if len(obj.GetFinalizers()) != 3 {
					t.Errorf("expected 3 finalizers, got: %v", len(obj.GetFinalizers()))
				}
				expectedFinalizers := []string{
					"first-test-finalizer",
					"second-test-finalizer",
					"third-test-finalizer",
				}
				if !reflect.DeepEqual(obj.GetFinalizers(), expectedFinalizers) {
					t.Errorf("expected finalizers '%+v' to match expectedFinalizers: '%+v'", obj.GetFinalizers(), expectedFinalizers)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RemoveFinalizer(tt.obj, tt.finalizer)
			tt.verify(tt.obj, t)
		})
	}
}

func Test_hasFinalizer(t *testing.T) {
	tests := []struct {
		name      string
		obj       metav1.Object
		finalizer string
		expect    bool
	}{
		{
			name: "existing finalizer found",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Finalizers: []string{
						"test-finalizer",
					},
				},
			},
			finalizer: "test-finalizer",
			expect:    true,
		},
		{
			name: "existing finalizer not found",
			obj: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-object",
					Finalizers: []string{
						"value",
					},
				},
			},
			finalizer: "test-key",
			expect:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasFinalizer(tt.obj, tt.finalizer)
			if !got == tt.expect {
				t.Errorf("expected '%v' got '%v'", tt.expect, got)
			}
		})
	}
}
