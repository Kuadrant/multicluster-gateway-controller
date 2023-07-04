package gracePeriod

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
)

func init() {
	if err := workv1.AddToScheme(scheme.Scheme); err != nil {
		panic(err)
	}
}

func TestGracefulDelete(t *testing.T) {
	now := time.Now()
	oneSecondAway := now.Add(time.Second)
	oneSecondAwayUnix := fmt.Sprint(oneSecondAway)
	cases := []struct {
		Name   string
		Object client.Object
		At     time.Time
		Verify func(t *testing.T, updatedObj client.Object, err, getErr error)
	}{
		{
			Name: "deleting in future sets expected annotation",
			Object: &workv1.ManifestWork{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "gateway-test-test",
					Namespace: "test",
				},
			},
			Verify: func(t *testing.T, updatedObj client.Object, err, getErr error) {
				if errors.Is(err, ErrGracePeriodNotExpired) {
					t.Fatalf("expected graceful delete error to be nil, got: %v", err)
				}
				if getErr != nil {
					t.Fatalf("expected get error to be 'nil', got '%v'", getErr)
				}
				if updatedObj.GetName() != "gateway-test-test" {
					t.Fatalf("expected updated object, got %v", updatedObj)
				}
				if metadata.GetAnnotation(updatedObj, GraceTimestampAnnotation) != oneSecondAwayUnix {
					t.Fatalf("expected grace timestamp: '%v' got '%v'", oneSecondAwayUnix, metadata.GetAnnotation(updatedObj, GraceTimestampAnnotation))
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			fc := fake.NewClientBuilder().WithObjects(tc.Object).Build()
			err := GracefulDelete(context.TODO(), fc, tc.Object)
			mw := &workv1.ManifestWork{}
			getErr := fc.Get(context.TODO(), client.ObjectKeyFromObject(tc.Object), mw)
			tc.Verify(t, mw, err, getErr)
		})
	}
}
