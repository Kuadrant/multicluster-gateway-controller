//go:build integration

package policy_integration

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	v1 "open-cluster-management.io/api/cluster/v1"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayv1beta1 "sigs.k8s.io/gateway-api/apis/v1beta1"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/conditions"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/_internal/metadata"
	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/dnspolicy"
	mgcgateway "github.com/Kuadrant/multicluster-gateway-controller/pkg/controllers/gateway"
	testutil "github.com/Kuadrant/multicluster-gateway-controller/test/util"
)

var _ = Describe("DNSPolicy", Ordered, func() {

	var gatewayClass *gatewayv1beta1.GatewayClass
	var managedZone *v1alpha1.ManagedZone
	var testNamespace string
	var dnsPolicyBuilder *testutil.DNSPolicyBuilder
	var gateway *gatewayv1beta1.Gateway
	var dnsPolicy *v1alpha1.DNSPolicy
	var recordName, wildcardRecordName string

	BeforeAll(func() {
		gatewayClass = testutil.NewTestGatewayClass("foo", "default", "kuadrant.io/bar")
		Expect(k8sClient.Create(ctx, gatewayClass)).To(BeNil())
		Eventually(func() error { // gateway class exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: gatewayClass.Name}, gatewayClass)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
	})

	BeforeEach(func() {
		CreateNamespace(&testNamespace)
		managedZone = testutil.NewManagedZoneBuilder("mz-example-com", testNamespace, "example.com").ManagedZone
		Expect(k8sClient.Create(ctx, managedZone)).To(BeNil())
		Eventually(func() error { // managed zone exists
			return k8sClient.Get(ctx, client.ObjectKey{Name: managedZone.Name, Namespace: managedZone.Namespace}, managedZone)
		}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		dnsPolicyBuilder = testutil.NewDNSPolicyBuilder("test-dns-policy", testNamespace)
	})

	AfterEach(func() {
		if gateway != nil {
			err := k8sClient.Delete(ctx, gateway)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
		if dnsPolicy != nil {
			err := k8sClient.Delete(ctx, dnsPolicy)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

		}
		if managedZone != nil {
			err := k8sClient.Delete(ctx, managedZone)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())
		}
	})

	AfterAll(func() {
		err := k8sClient.Delete(ctx, gatewayClass)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("invalid target", func() {

		BeforeEach(func() {
			dnsPolicy = dnsPolicyBuilder.
				WithTargetGateway("test-gateway").
				WithRoutingStrategy(v1alpha1.SimpleRoutingStrategy).
				DNSPolicy
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())
			Eventually(func() error { //dns policy exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())
		})

		It("should have ready condition with status false and correct reason", func() {
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				readyCond := meta.FindStatusCondition(dnsPolicy.Status.Conditions, string(conditions.ConditionTypeReady))
				if readyCond == nil {
					return fmt.Errorf("expected dnsPolicy to have %s condition, got none",
						string(conditions.ConditionTypeReady))
				}
				if readyCond.Status != metav1.ConditionFalse {
					return fmt.Errorf("expected dnsPolicy %s condition to have status %s, got %s",
						string(conditions.ConditionTypeReady), metav1.ConditionFalse, readyCond.Status)
				}
				if readyCond.Reason != string(conditions.PolicyReasonTargetNotFound) {
					return fmt.Errorf("expected dnsPolicy %s condition to have reason %s, got %s",
						string(conditions.ConditionTypeReady), string(conditions.PolicyReasonTargetNotFound), readyCond.Reason)
				}
				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

		It("should have ready condition with status true", func() {
			By("creating a valid Gateway")

			gateway = testutil.NewGatewayBuilder("test-gateway", gatewayClass.Name, testNamespace).
				WithHTTPListener("test-listener", "test.example.com").Gateway
			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Eventually(func() error { //gateway exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				if !meta.IsStatusConditionTrue(dnsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
					return fmt.Errorf("expected dnsPolicy %s condition to have status %s ", string(conditions.ConditionTypeReady), metav1.ConditionTrue)
				}
				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

		It("should not have any health check records created", func() {
			// create a health check with the labels for the dnspolicy and the gateway name and namespace that would be expected in a valid target scenario
			// this one should get deleted if the gateway is invalid policy ref
			probe := &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s-%s", TestIPAddressTwo, TestGatewayName, TestHostOne),
					Namespace: testNamespace,
					Labels: map[string]string{
						DNSPolicyBackRefAnnotation:                              "test-dns-policy",
						fmt.Sprintf("%s-namespace", DNSPolicyBackRefAnnotation): testNamespace,
						LabelGatewayNSRef:                                       testNamespace,
						LabelGatewayReference:                                   "test-gateway",
					},
				},
			}
			Expect(k8sClient.Create(ctx, probe)).To(BeNil())

			Eventually(func() error { // probe should be present
				getCreatedProbe := &v1alpha1.DNSHealthCheckProbe{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: probe.Name, Namespace: probe.Namespace}, getCreatedProbe)
				return err
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			Eventually(func() bool { // probe should be removed
				getDeletedProbe := &v1alpha1.DNSHealthCheckProbe{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: probe.Name, Namespace: probe.Namespace}, getDeletedProbe)
				if err != nil {
					if k8serrors.IsNotFound(err) {
						return true
					}
				}
				return false
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeTrue())
		})

	})

	Context("valid target with no gateway status", func() {
		testGatewayName := "test-no-gateway-status"

		BeforeEach(func() {
			gateway = testutil.NewGatewayBuilder(testGatewayName, gatewayClass.Name, testNamespace).
				WithHTTPListener(TestListenerNameOne, TestHostOne).
				Gateway
			dnsPolicy = dnsPolicyBuilder.
				WithTargetGateway(testGatewayName).
				WithRoutingStrategy(v1alpha1.SimpleRoutingStrategy).
				DNSPolicy

			Expect(k8sClient.Create(ctx, gateway)).To(BeNil())
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(BeNil())

			Eventually(func() error { //gateway exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			Eventually(func() error { //dns policy exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
		})

		It("should not create a dns record", func() {
			Consistently(func() []v1alpha1.DNSRecord { // DNS record exists
				dnsRecords := v1alpha1.DNSRecordList{}
				err := k8sClient.List(ctx, &dnsRecords, client.InNamespace(dnsPolicy.GetNamespace()))
				Expect(err).ToNot(HaveOccurred())
				return dnsRecords.Items
			}, time.Second*15, time.Second).Should(BeEmpty())
		})

		It("should have ready status", func() {
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				if !meta.IsStatusConditionTrue(dnsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
					return fmt.Errorf("expected DNSPolicy status condition to be %s", string(conditions.ConditionTypeReady))
				}
				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

		It("should set gateway back reference", func() {
			existingGateway := &gatewayv1beta1.Gateway{}
			policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
			refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
			policiesBackRefValue := string(refs)
			Eventually(func() error {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				if err != nil {
					return err
				}
				annotations := existingGateway.GetAnnotations()
				if annotations == nil {
					return fmt.Errorf("existingGateway annotations should not be nil")
				}
				if _, ok := annotations[DNSPolicyBackRefAnnotation]; !ok {
					return fmt.Errorf("existingGateway annotations do not have annotation %s", DNSPolicyBackRefAnnotation)
				}
				if annotations[DNSPolicyBackRefAnnotation] != policyBackRefValue {
					return fmt.Errorf("existingGateway annotations[%s] does not have expected value", DNSPolicyBackRefAnnotation)
				}
				return nil
			}, time.Second*5, time.Second).Should(BeNil())
			Eventually(func() error {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				if err != nil {
					return err
				}
				annotations := existingGateway.GetAnnotations()
				if annotations == nil {
					return fmt.Errorf("existingGateway annotations should not be nil")
				}
				if _, ok := annotations[DNSPoliciesBackRefAnnotation]; !ok {
					return fmt.Errorf("existingGateway annotations do not have annotation %s", DNSPoliciesBackRefAnnotation)
				}
				if annotations[DNSPoliciesBackRefAnnotation] != policiesBackRefValue {
					return fmt.Errorf("existingGateway annotations[%s] does not have expected value", DNSPoliciesBackRefAnnotation)
				}
				return nil
			}, time.Second*5, time.Second).Should(BeNil())
		})
	})

	Context("valid target and valid gateway status", func() {

		BeforeEach(func() {
			gateway = testutil.NewGatewayBuilder(TestGatewayName, gatewayClass.Name, testNamespace).
				WithHTTPListener(TestListenerNameOne, TestHostOne).
				WithHTTPListener(TestListenerNameWildcard, TestHostWildcard).
				Gateway
			dnsPolicy = dnsPolicyBuilder.WithTargetGateway(TestGatewayName).
				WithRoutingStrategy(v1alpha1.SimpleRoutingStrategy).
				//WithLoadBalancingWeightedFor(120, nil).
				DNSPolicy

			Expect(k8sClient.Create(ctx, gateway)).To(Succeed())
			Expect(k8sClient.Create(ctx, dnsPolicy)).To(Succeed())

			Eventually(func() error { //dns policy exists
				return k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy)
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			Eventually(func() error {
				if err := k8sClient.Create(ctx, &v1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: TestClusterNameOne,
					},
				}); err != nil && !k8serrors.IsAlreadyExists(err) {
					return err
				}
				if err := k8sClient.Create(ctx, &v1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: TestClusterNameTwo,
					},
				}); err != nil && !k8serrors.IsAlreadyExists(err) {
					return err
				}

				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, gateway)
				Expect(err).ShouldNot(HaveOccurred())

				gateway.Status.Addresses = []gatewayv1beta1.GatewayAddress{
					{
						Type:  testutil.Pointer(mgcgateway.MultiClusterIPAddressType),
						Value: TestClusterNameOne + "/" + TestIPAddressOne,
					},
					{
						Type:  testutil.Pointer(mgcgateway.MultiClusterIPAddressType),
						Value: TestClusterNameTwo + "/" + TestIPAddressTwo,
					},
				}
				gateway.Status.Listeners = []gatewayv1beta1.ListenerStatus{
					{
						Name:           TestClusterNameOne + "." + TestListenerNameOne,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
					{
						Name:           TestClusterNameTwo + "." + TestListenerNameOne,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
					{
						Name:           TestClusterNameOne + "." + TestListenerNameWildcard,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
					{
						Name:           TestClusterNameTwo + "." + TestListenerNameWildcard,
						SupportedKinds: []gatewayv1beta1.RouteGroupKind{},
						AttachedRoutes: 1,
						Conditions:     []metav1.Condition{},
					},
				}
				return k8sClient.Status().Update(ctx, gateway)
			}, TestTimeoutMedium, TestRetryIntervalMedium).ShouldNot(HaveOccurred())

			recordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameOne)
			wildcardRecordName = fmt.Sprintf("%s-%s", TestGatewayName, TestListenerNameWildcard)
		})

		It("should have correct status", func() {
			Eventually(func() error {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: dnsPolicy.Namespace}, dnsPolicy); err != nil {
					return err
				}
				if !meta.IsStatusConditionTrue(dnsPolicy.Status.Conditions, string(conditions.ConditionTypeReady)) {
					return fmt.Errorf("expected status condition %s to be True", conditions.ConditionTypeReady)
				}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(gateway), gateway); err != nil {
					return err
				}

				policyAffectedCond := meta.FindStatusCondition(gateway.Status.Conditions, string(DNSPolicyAffected))
				if policyAffectedCond == nil {
					return fmt.Errorf("policy affected conditon expected but not found")
				}
				if policyAffectedCond.ObservedGeneration != gateway.Generation {
					return fmt.Errorf("expected policy affected cond generation to be %d but got %d", gateway.Generation, policyAffectedCond.ObservedGeneration)
				}
				if !meta.IsStatusConditionTrue(gateway.Status.Conditions, string(DNSPolicyAffected)) {
					return fmt.Errorf("expected gateway status condition %s to be True", DNSPolicyAffected)
				}

				return nil
			}, time.Second*15, time.Second).Should(BeNil())
		})

		It("should set gateway back reference", func() {
			existingGateway := &gatewayv1beta1.Gateway{}
			policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
			refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
			policiesBackRefValue := string(refs)

			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPolicyBackRefAnnotation, policyBackRefValue))
			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))
		})

		It("should remove dns records when listener removed", func() {
			//get the gateway and remove the listeners

			Eventually(func() error {
				existingGateway := &gatewayv1beta1.Gateway{}
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(gateway), existingGateway); err != nil {
					return err
				}
				newListeners := []gatewayv1beta1.Listener{}
				for _, existing := range existingGateway.Spec.Listeners {
					if existing.Name == TestListenerNameWildcard {
						newListeners = append(newListeners, existing)
					}
				}

				patch := client.MergeFrom(existingGateway.DeepCopy())
				existingGateway.Spec.Listeners = newListeners
				rec := &v1alpha1.DNSRecord{}
				if err := k8sClient.Patch(ctx, existingGateway, patch); err != nil {
					return err
				}
				//dns record should be removed for non wildcard
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, rec); err != nil && !k8serrors.IsNotFound(err) {
					return err
				}
				return k8sClient.Get(ctx, client.ObjectKey{Name: wildcardRecordName, Namespace: testNamespace}, rec)
			}, time.Second*10, time.Second).Should(BeNil())
		})

		It("should remove gateway back reference on policy deletion", func() {
			existingGateway := &gatewayv1beta1.Gateway{}
			policyBackRefValue := testNamespace + "/" + dnsPolicy.Name
			refs, _ := json.Marshal([]client.ObjectKey{{Name: dnsPolicy.Name, Namespace: testNamespace}})
			policiesBackRefValue := string(refs)

			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPolicyBackRefAnnotation, policyBackRefValue))
			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).Should(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))

			//finalizer should exist
			Eventually(func() bool {
				existingDNSPolicy := &v1alpha1.DNSPolicy{}
				err := k8sClient.Get(ctx, client.ObjectKey{Name: dnsPolicy.Name, Namespace: testNamespace}, existingDNSPolicy)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return metadata.HasFinalizer(existingDNSPolicy, DNSPolicyFinalizer)
			}, time.Second*5, time.Second).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, dnsPolicy)).To(BeNil())

			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: gateway.Namespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).ShouldNot(HaveKey(DNSPolicyBackRefAnnotation))

			Eventually(func() map[string]string {
				// Check gateway back references
				err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway)
				// must exist
				Expect(err).ToNot(HaveOccurred())
				return existingGateway.GetAnnotations()
			}, time.Second*5, time.Second).ShouldNot(HaveKeyWithValue(DNSPoliciesBackRefAnnotation, policiesBackRefValue))

			Eventually(func() error {
				// Check gateway back references
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: gateway.Name, Namespace: testNamespace}, existingGateway); err != nil {
					return err
				}
				cond := meta.FindStatusCondition(existingGateway.Status.Conditions, string(DNSPolicyAffected))
				if cond != nil {
					return fmt.Errorf("expected the condition %s to be gone", DNSPolicyAffected)
				}
				return nil
			}, time.Second*5, time.Second).Should(BeNil())
		})

		It("should remove dns record reference on policy deletion even if gateway is removed", func() {
			createdDNSRecord := &v1alpha1.DNSRecord{}
			Eventually(func() error { // DNS record exists
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, createdDNSRecord); err != nil {
					return err
				}
				return nil
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())

			err := k8sClient.Delete(ctx, gateway)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

			err = k8sClient.Delete(ctx, dnsPolicy)
			Expect(client.IgnoreNotFound(err)).ToNot(HaveOccurred())

			Eventually(func() error { // DNS record removed
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: recordName, Namespace: testNamespace}, createdDNSRecord); err != nil {
					if k8serrors.IsNotFound(err) {
						return nil
					}
					return err
				}
				return errors.New("found dnsrecord when it should be deleted")
			}, TestTimeoutMedium, TestRetryIntervalMedium).Should(BeNil())
		})

	})

})
