//go:build integration

// /*
// Copyright 2023 The MultiCluster Traffic Controller Authors.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// */

package integration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("ManagedZoneReconciler", func() {
	Context("testing ManagedZone controller", func() {
		var managedZone *v1alpha1.ManagedZone

		BeforeEach(func() {
			managedZone = &v1alpha1.ManagedZone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example.com",
					Namespace: "default",
				},
				Spec: v1alpha1.ManagedZoneSpec{
					ID:         "example.com",
					DomainName: "example.com",
					SecretRef: &v1alpha1.SecretRef{
						Name:      providerCredential,
						Namespace: "default",
					},
				},
			}
		})

		AfterEach(func() {
			// Clean up managedZones
			mzList := &v1alpha1.ManagedZoneList{}
			err := k8sClient.List(ctx, mzList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, mz := range mzList.Items {
				err = k8sClient.Delete(ctx, &mz)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should accept a managed zone for this controller and allow deletion", func() {
			Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())

			createdMZ := &v1alpha1.ManagedZone{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: managedZone.Namespace, Name: managedZone.Name}, createdMZ)
				return err == nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, managedZone)).To(BeNil())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: managedZone.Namespace, Name: managedZone.Name}, createdMZ)
				return errors.IsNotFound(err)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should reject a managed zone with an invalid domain name", func() {
			invalidDomainNameManagedZone := &v1alpha1.ManagedZone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid_domain",
					Namespace: "default",
				},
				Spec: v1alpha1.ManagedZoneSpec{
					ID:         "invalid_domain",
					DomainName: "invalid_domain",
				},
			}
			err := k8sClient.Create(ctx, invalidDomainNameManagedZone)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("spec.domainName in body should match"))
		})
	})
})
