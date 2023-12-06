//go:build integration

package policy_integration

import (
	"context"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/gomega"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func CreateNamespace(namespace *string) {
	var generatedTestNamespace = "test-namespace-" + uuid.New().String()

	nsObject := &v1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: generatedTestNamespace},
	}

	err := testClient().Create(context.Background(), nsObject)
	Expect(err).ToNot(HaveOccurred())

	existingNamespace := &v1.Namespace{}
	Eventually(func() error {
		return testClient().Get(context.Background(), types.NamespacedName{Name: generatedTestNamespace}, existingNamespace)
	}, time.Minute, 5*time.Second).ShouldNot(HaveOccurred())

	*namespace = existingNamespace.Name
}

func DeleteNamespace(namespace *string) {
	desiredTestNamespace := &v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: *namespace}}
	err := testClient().Delete(context.Background(), desiredTestNamespace, client.PropagationPolicy(metav1.DeletePropagationForeground))

	Expect(err).ToNot(HaveOccurred())

	existingNamespace := &v1.Namespace{}
	Eventually(func() error {
		err := testClient().Get(context.Background(), types.NamespacedName{Name: *namespace}, existingNamespace)
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return nil
	}, 3*time.Minute, 2*time.Second).Should(BeNil())
}
