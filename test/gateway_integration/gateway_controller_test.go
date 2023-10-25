//go:build integration

/*
Copyright 2023 The MultiCluster Traffic Controller Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gateway_integration

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ocmclusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	ocmworkv1 "open-cluster-management.io/api/work/v1"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/test/util"
	//+kubebuilder:scaffold:imports
)

var _ = Describe("GatewayClassController", func() {
	Context("testing gatewayclass controller", func() {
		var gatewayclass *gatewayv1beta1.GatewayClass
		BeforeEach(func() {
			gatewayclass = &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Namespace: "default",
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mgc-gw-controller",
				},
			}
		})

		AfterEach(func() {
			// Clean up GatewayClasses
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err := k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gatewayclass := range gatewayclassList.Items {
				err = k8sClient.Delete(ctx, &gatewayclass)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		It("should accept a gatewayclass for this controller", func() {
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() error {
				return k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			// Status Accepted
			var condition metav1.Condition
			Eventually(func() error {
				if err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass); err != nil {
					return err
				}
				if createdGatewayclass.Status.Conditions[0].Status != metav1.ConditionTrue {
					return fmt.Errorf("expected createdGatewayclass condition to not be true, got %v", createdGatewayclass.Status.Conditions[0].Status)
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
			condition = createdGatewayclass.Status.Conditions[0]
			Expect(len(createdGatewayclass.Status.Conditions)).To(BeEquivalentTo(1))
			Expect(condition.Type).To(BeEquivalentTo(gatewayv1beta1.GatewayClassConditionStatusAccepted))
			Expect(condition.Reason).To(BeEquivalentTo(gatewayv1beta1.GatewayClassConditionStatusAccepted))
		})

		It("should NOT accept a gatewayclass for a different controller", func() {
			gatewayclass.Name = "some-other-gatewayclass"
			gatewayclass.Spec.ControllerName = "example.com/some-other-controller"
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() error {
				return k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			// Only 1
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err := k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			Expect(len(gatewayclassList.Items)).To(BeEquivalentTo(1))

			// Status stays Unknown
			var condition metav1.Condition
			Consistently(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				condition = createdGatewayclass.Status.Conditions[0]
				return condition.Type == string(gatewayv1beta1.GatewayClassConditionStatusAccepted) && condition.Status == metav1.ConditionUnknown
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

		It("should NOT accept a gatewayclass that is 'unsupported'", func() {
			gatewayclass.Name = "test-class-name-1"
			Expect(k8sClient.Create(ctx, gatewayclass)).To(BeNil())
			createdGatewayclass := &gatewayv1beta1.GatewayClass{}
			gatewayclassType := types.NamespacedName{Name: gatewayclass.Name, Namespace: gatewayclass.Namespace}

			// Exists
			Eventually(func() error {
				return k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			// Status is false
			var condition metav1.Condition
			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayclassType, createdGatewayclass)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					log.Log.Error(err, "No errors expected")
					Fail("No errors expected")
				}
				condition = createdGatewayclass.Status.Conditions[0]
				return condition.Type == string(gatewayv1beta1.GatewayClassConditionStatusAccepted) && condition.Status == metav1.ConditionFalse
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
			Expect(condition.Reason).To(BeEquivalentTo(gatewayv1beta1.GatewayClassReasonInvalidParameters))
			Expect(condition.Message).To(BeEquivalentTo("Invalid Parameters - Unsupported class name test-class-name-1. Must be one of [kuadrant-multi-cluster-gateway-instance-per-cluster]"))
		})
	})
})

var _ = Describe("GatewayController", func() {
	Context("testing gateway controller", func() {
		var gateway *gatewayv1beta1.Gateway
		var noLGateway *gatewayv1beta1.Gateway
		var gatewayClass *gatewayv1beta1.GatewayClass
		var managedZone *v1alpha1.ManagedZone
		var manifest1 *ocmworkv1.ManifestWork
		var manifest2 *ocmworkv1.ManifestWork
		var nsSpoke1 *corev1.Namespace
		var nsSpoke2 *corev1.Namespace
		var placementDecision *ocmclusterv1beta1.PlacementDecision
		var hostnametest gatewayv1beta1.Hostname
		var stringified string

		//Kicks off before the tests run, used to create resources we need in multiple tests,were created in a diff function that we are not testing but need the output, or things a user may have to do.
		BeforeEach(func() {
			// Before: Create GatewayClass for the gateway
			gatewayClass = &gatewayv1beta1.GatewayClass{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Namespace: defaultNS,
				},
				Spec: gatewayv1beta1.GatewayClassSpec{
					ControllerName: "kuadrant.io/mctc-gw-controller",
				},
			}
			Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())

			// Before: Create a test ManagedZone for test Gateway listeners to use
			managedZone = &v1alpha1.ManagedZone{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "example.com",
					Namespace: defaultNS,
				},
				Spec: v1alpha1.ManagedZoneSpec{
					ID:          "1234",
					DomainName:  "example.com",
					Description: "example.com",
					SecretRef: &v1alpha1.SecretRef{
						Name:      providerCredential,
						Namespace: defaultNS,
					},
				},
			}

			Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())

			//Before: Create placement decision

			placementDecision = &ocmclusterv1beta1.PlacementDecision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-placement",
					Namespace: defaultNS,
					Labels: map[string]string{
						"cluster.open-cluster-management.io/placement": "GatewayControllerTest",
					},
				},
			}

			Expect(k8sClient.Create(ctx, placementDecision)).To(BeNil())

			// Before: Update the statsus of theplacement decision to include the clusters we want to place as OCM would usually do this
			placementDecision.Status = ocmclusterv1beta1.PlacementDecisionStatus{
				Decisions: []ocmclusterv1beta1.ClusterDecision{
					{
						ClusterName: nsSpoke1Name,
						Reason:      "test",
					},
					{
						ClusterName: nsSpoke2Name,
						Reason:      "test",
					},
				},
			}
			Expect(k8sClient.Status().Update(ctx, placementDecision)).To(BeNil())
			//Before: Stub Gateway for tests but not creating it here
			hostname1 := gatewayv1beta1.Hostname("test1.example.com")
			gateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gw-1",
					Namespace: defaultNS,
					Labels: map[string]string{
						"cluster.open-cluster-management.io/placement": "GatewayControllerTest",
					},
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "default",
							Port:     8443,
							Protocol: gatewayv1beta1.HTTPSProtocolType,
							Hostname: &hostname1,
						},
					},
				},
			}

			noLGateway = &gatewayv1beta1.Gateway{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gw-2",
					Namespace: defaultNS,
				},
				Spec: gatewayv1beta1.GatewaySpec{
					GatewayClassName: "kuadrant-multi-cluster-gateway-instance-per-cluster",
					Listeners: []gatewayv1beta1.Listener{
						{
							Name:     "default",
							Port:     8443,
							Protocol: gatewayv1beta1.HTTPSProtocolType,
							Hostname: &hostname1,
						},
					},
				},
			}
			// Before: Create namespace 1 to mock a spoke cluster
			nsSpoke1 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsSpoke1Name,
				},
			}
			err := k8sClient.Create(ctx, nsSpoke1)
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())

			}
			//Before: Create namespace 2 to mock a 2nd spoke cluster
			nsSpoke2 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsSpoke2Name,
				},
			}

			err = k8sClient.Create(ctx, nsSpoke2)
			if err != nil && !k8serrors.IsAlreadyExists(err) {
				Expect(err).ToNot(HaveOccurred())
			}

		})
		// Occurs after the test is complete
		AfterEach(func() {
			// Clean: Gateways
			gatewayList := &gatewayv1beta1.GatewayList{}
			err := k8sClient.List(ctx, gatewayList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gateway := range gatewayList.Items {
				err = k8sClient.Delete(ctx, &gateway)
				Expect(err).NotTo(HaveOccurred())
			}

			// Clean: GatewayClasses
			gatewayclassList := &gatewayv1beta1.GatewayClassList{}
			err = k8sClient.List(ctx, gatewayclassList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, gatewayclass := range gatewayclassList.Items {
				err = k8sClient.Delete(ctx, &gatewayclass)
				Expect(err).NotTo(HaveOccurred())
			}

			// Clean: ManagedZones
			managedZoneList := &v1alpha1.ManagedZoneList{}
			err = k8sClient.List(ctx, managedZoneList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, managedZone := range managedZoneList.Items {
				err = k8sClient.Delete(ctx, &managedZone)
				Expect(err).NotTo(HaveOccurred())
			}
			// Clean: Placement decisions
			placementDecisionList := &ocmclusterv1beta1.PlacementDecisionList{}
			err = k8sClient.List(ctx, placementDecisionList, client.InNamespace("default"))
			Expect(err).NotTo(HaveOccurred())
			for _, placementDecision := range placementDecisionList.Items {
				err = k8sClient.Delete(ctx, &placementDecision)
				Expect(err).NotTo(HaveOccurred())
			}
		})

		// This tests the full reconcile of a gateway from start to finish
		It("should reconcile a gateway", func() {
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			upstreamGateway := &gatewayv1beta1.Gateway{}
			upstreamGatewayType := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}
			manifest1 = &ocmworkv1.ManifestWork{}
			manifest2 = &ocmworkv1.ManifestWork{}

			// Test: Passes if it gets the gateway
			Eventually(func() error {
				return k8sClient.Get(ctx, upstreamGatewayType, upstreamGateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			// Test: Passes if the gateway contains a finalizer
			Eventually(func() error {
				if err := k8sClient.Get(ctx, upstreamGatewayType, upstreamGateway); err != nil {
					return err
				}
				if !controllerutil.ContainsFinalizer(upstreamGateway, gatewayFinalizer) {
					return fmt.Errorf("expected finalizer %s in upstreamGateway", gatewayFinalizer)
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			// Test: Passes when manifest1 is found in Namespace test-spoke-cluster-1 and contains the hostname from the gateway
			Eventually(func() error {
				mwList := ocmworkv1.ManifestWorkList{}
				if err := k8sClient.List(ctx, &mwList, &client.ListOptions{Namespace: "nsSpoke1Name"}); err != nil {
					log.Log.Error(err, "error getting ManifestWork")
					return err
				}
				log.Log.Info("manifests", "items", mwList.Items)
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nsSpoke1Name, Name: "gateway-default-test-gw-1"}, manifest1); err != nil {
					log.Log.Error(err, "error getting ManifestWork")
					return err
				}
				// decoding the raw format in the manifest 1 into a readable variable that can be compared
				rawBytes := manifest1.Spec.Workload.Manifests[0].Raw
				gateway := &gatewayv1beta1.Gateway{}
				err := json.Unmarshal(rawBytes, gateway)
				if err != nil {
					log.Log.Error(err, "failed to unmarshal gateway")
					return err
				}
				hostnametest = *gateway.Spec.Listeners[0].Hostname
				stringified = string(hostnametest)
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
			//Comparing the hostname
			Expect(stringified).To(Equal("test1.example.com"))

			// Test: Passes when manifest2 is found in Namespace test-spoke-cluster-2 and contains the hostname from the gateway

			Eventually(func() error {
				if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nsSpoke2Name, Name: "gateway-default-test-gw-1"}, manifest2); err != nil {
					log.Log.Error(err, "error getting ManifestWork")
					return err
				}
				// decoding the raw format in the manifest 1 into a readable variable that can be compared
				rawBytes := manifest2.Spec.Workload.Manifests[0].Raw
				gateway := &gatewayv1beta1.Gateway{}
				err := json.Unmarshal(rawBytes, gateway)
				if err != nil {
					return err
				}
				hostnametest = *gateway.Spec.Listeners[0].Hostname
				stringified = string(hostnametest)
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			//Comparing the hostname
			Expect(stringified).To(Equal("test1.example.com"))

			//Mock: Make the manifestwork status in manifest 1 be "applied". OCM usually does it when the resources have been applied
			ipAddressType := gatewayv1beta1.IPAddressType
			hostnameAddressType := gatewayv1beta1.HostnameAddressType
			ip1 := "172.18.0.1"
			ip2 := "172.18.0.2"
			ip3 := "172.18.1.1"
			hostname4 := "test4.example.com"
			m1AddressesJson, err := json.Marshal([]gatewayv1beta1.GatewayAddress{
				{
					Type:  &ipAddressType,
					Value: ip1,
				},
				{
					Type:  &ipAddressType,
					Value: ip2,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			m1AddressesJsonString := string(m1AddressesJson)
			manifest1.Status = ocmworkv1.ManifestWorkStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Applied",
						Status:             "True",
						LastTransitionTime: metav1.Now(),
						Reason:             "AppliedManifestComplete",
						Message:            "Apply manifest complete",
					},
				},
				ResourceStatus: ocmworkv1.ManifestResourceStatus{
					Manifests: []ocmworkv1.ManifestCondition{
						{
							ResourceMeta: ocmworkv1.ManifestResourceMeta{
								Group:     "gateway.networking.k8s.io",
								Name:      gateway.Name,
								Namespace: gateway.Namespace,
							},
							StatusFeedbacks: ocmworkv1.StatusFeedbackResult{
								Values: []ocmworkv1.FeedbackValue{
									{
										Name: "addresses",
										Value: ocmworkv1.FieldValue{
											Type:    ocmworkv1.JsonRaw,
											JsonRaw: &m1AddressesJsonString,
										},
									},
									{
										Name: "listenerdefaultAttachedRoutes",
										Value: ocmworkv1.FieldValue{
											Type:    ocmworkv1.Integer,
											Integer: Pointer(int64(1)),
										},
									},
									{
										Name: "listenerwebAttachedRoutes",
										Value: ocmworkv1.FieldValue{
											Type:    ocmworkv1.Integer,
											Integer: Pointer(int64(1)),
										},
									},
								},
							},
						},
					},
				},
			}
			Eventually(func() error {
				return k8sClient.Status().Update(ctx, manifest1)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: manifest2.Namespace, Name: manifest2.Name}, manifest2)).To(BeNil())
			//Mock: Make the manifestwork status in manifest 2 be "applied". OCM usually does it when the resources have been applied
			m2AddressesJson, err := json.Marshal([]gatewayv1beta1.GatewayAddress{
				{
					Type:  &ipAddressType,
					Value: ip3,
				},
				{
					Type:  &hostnameAddressType,
					Value: hostname4,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			m2AddressesJsonString := string(m2AddressesJson)
			manifest2.Status = ocmworkv1.ManifestWorkStatus{
				Conditions: []metav1.Condition{
					{
						Type:               "Applied",
						Status:             "True",
						LastTransitionTime: metav1.Now(),
						Reason:             "AppliedManifestComplete",
						Message:            "Apply manifest complete",
					},
				},
				ResourceStatus: ocmworkv1.ManifestResourceStatus{
					Manifests: []ocmworkv1.ManifestCondition{
						{
							ResourceMeta: ocmworkv1.ManifestResourceMeta{
								Group:     "gateway.networking.k8s.io",
								Name:      gateway.Name,
								Namespace: gateway.Namespace,
							},
							StatusFeedbacks: ocmworkv1.StatusFeedbackResult{
								Values: []ocmworkv1.FeedbackValue{
									{
										Name: "addresses",
										Value: ocmworkv1.FieldValue{
											Type:    ocmworkv1.JsonRaw,
											JsonRaw: &m2AddressesJsonString,
										},
									},
									{
										Name: "listenerdefaultAttachedRoutes",
										Value: ocmworkv1.FieldValue{
											Type:    ocmworkv1.Integer,
											Integer: Pointer(int64(0)),
										},
									},
									{
										Name: "listenerwebAttachedRoutes",
										Value: ocmworkv1.FieldValue{
											Type:    ocmworkv1.Integer,
											Integer: Pointer(int64(1)),
										},
									},
								},
							},
						},
					},
				},
			}

			Expect(k8sClient.Status().Update(ctx, manifest2)).To(BeNil())

			// Test that the Programmed condition is True
			var programmedCondition *metav1.Condition
			Eventually(func() error {
				err := k8sClient.Get(ctx, upstreamGatewayType, upstreamGateway)
				if err != nil {
					// explicitly fail as we should be passed the point of any errors
					return fmt.Errorf("No errors expected: %v", err)
				}
				programmedCondition = meta.FindStatusCondition(upstreamGateway.Status.Conditions, string(gatewayv1beta1.GatewayConditionProgrammed))
				log.Log.Info("programmedCondition", "programmedCondition", programmedCondition)
				Expect(programmedCondition).ToNot(BeNil())
				if programmedCondition.Status != metav1.ConditionTrue {
					return fmt.Errorf("programmedCondition is not true, got %v", programmedCondition.Status)
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			// Test the aggregated addresses are correct
			addresses := upstreamGateway.Status.Addresses
			Expect(len(addresses)).To(BeEquivalentTo(4)) // 2 Gateways. Each Gateway has 2 IPAddresses
			address1 := addresses[0]
			Expect(string(*address1.Type)).To(BeEquivalentTo("kuadrant.io/MultiClusterIPAddress"))
			Expect(address1.Value).To(BeEquivalentTo(fmt.Sprintf("%s/%s", nsSpoke1Name, ip1)))
			address2 := addresses[1]
			Expect(string(*address2.Type)).To(BeEquivalentTo("kuadrant.io/MultiClusterIPAddress"))
			Expect(address2.Value).To(BeEquivalentTo(fmt.Sprintf("%s/%s", nsSpoke1Name, ip2)))
			address3 := addresses[2]
			Expect(string(*address3.Type)).To(BeEquivalentTo("kuadrant.io/MultiClusterIPAddress"))
			Expect(address3.Value).To(BeEquivalentTo(fmt.Sprintf("%s/%s", nsSpoke2Name, ip3)))
			address4 := addresses[3]
			Expect(string(*address4.Type)).To(BeEquivalentTo("kuadrant.io/MultiClusterHostnameAddress"))
			Expect(address4.Value).To(BeEquivalentTo(fmt.Sprintf("%s/%s", nsSpoke2Name, hostname4)))

			// Test the aggregated listeners are correct
			listeners := upstreamGateway.Status.Listeners
			Expect(len(listeners)).To(BeEquivalentTo(2)) // 2 Gateways. Each Gateway has 1 listeners: 'default'
			listener1 := listeners[0]
			Expect(listener1.Name).To(BeEquivalentTo(fmt.Sprintf("%s.default", nsSpoke1Name)))
			Expect(listener1.AttachedRoutes).To(BeEquivalentTo(1))
			listener2 := listeners[1]
			Expect(listener2.Name).To(BeEquivalentTo(fmt.Sprintf("%s.default", nsSpoke2Name)))
			Expect(listener2.AttachedRoutes).To(BeEquivalentTo(0))
		})

		// Tests if the placement label isnt present no manifest should be created
		It("shouldnt place a gateway", func() {

			Expect(k8sClient.Create(ctx, noLGateway)).To(BeNil())
			noLabelGateway := &gatewayv1beta1.Gateway{}
			noLabelUpstreamGatewayType := types.NamespacedName{Name: noLGateway.Name, Namespace: noLGateway.Namespace}

			// Passes if it gets the gateway
			Eventually(func() error {
				return k8sClient.Get(ctx, noLabelUpstreamGatewayType, noLabelGateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			// Passes if the gateway contains a finalizer
			Eventually(func() error {
				if err := k8sClient.Get(ctx, noLabelUpstreamGatewayType, noLabelGateway); err != nil {
					return err
				}
				if !controllerutil.ContainsFinalizer(noLabelGateway, gatewayFinalizer) {
					return fmt.Errorf("expected finalizer %s in upstreamGateway", gatewayFinalizer)
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			// Test: Passes if no manifest was gotten
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Namespace: nsSpoke1Name, Name: "gateway-default-test-gw-2"}, manifest1)
				if err != nil && !k8serrors.IsNotFound(err) {
					return err
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

		})
	})

})
