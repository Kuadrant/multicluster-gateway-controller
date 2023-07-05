package dnshealthcheckprobe

import (
	"context"
	"time"

	"github.com/Kuadrant/multicluster-gateway-controller/pkg/apis/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DNSHealthCheckProbe controller", func() {
	const (
		ProbeName      = "test-probe"
		ProbeNamespace = "default"

		timeout  = time.Second * 10
		duration = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When creating DNSHealthCheckProbe", func() {
		It("Should update health status to healthy", func() {
			By("Performing health check")

			ctx := context.Background()
			probeObj := &v1alpha1.DNSHealthCheckProbe{
				ObjectMeta: metav1.ObjectMeta{
					Name:      ProbeName,
					Namespace: ProbeNamespace,
				},
				Spec: v1alpha1.DNSHealthCheckProbeSpec{
					Host:      "localhost",
					IPAddress: "0.0.0.0",
					Port:      3333,
					Interval:  metav1.Duration{Duration: time.Second * 10},
					Path:      "/healthy",
				},
			}

			Expect(k8sClient.Create(ctx, probeObj)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(probeObj), probeObj)
				if err != nil {
					GinkgoWriter.Print(err)
					return false
				}
				return probeObj.Status.LastCheckedAt.Time != metav1.Time{}.Time
			}, timeout+(time.Second*20), interval).Should(BeTrue())

			GinkgoWriter.Print(probeObj)

			Expect(probeObj.Status.Healthy).Should(BeTrue())
			Expect(probeObj.Status.LastCheckedAt).Should(Not(BeZero()))
		})
		It("Should update health status to unhealthy", func() {
			By("Updating to unhealthy endpoint")

			ctx := context.Background()
			probeObj := &v1alpha1.DNSHealthCheckProbe{}

			err := k8sClient.Get(ctx, client.ObjectKey{
				Name:      ProbeName,
				Namespace: ProbeNamespace,
			}, probeObj)
			Expect(err).NotTo(HaveOccurred())

			lastUpdate := probeObj.Status.LastCheckedAt
			probeObj.Spec.Path = "/unhealthy"
			err = k8sClient.Update(ctx, probeObj)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, client.ObjectKeyFromObject(probeObj), probeObj)
				if err != nil {
					GinkgoWriter.Print(err)
					return false
				}
				return probeObj.Status.LastCheckedAt.Time.After(lastUpdate.Time)
			}, timeout+(time.Second*20), interval).Should(BeTrue())

			Expect(probeObj.Status.Healthy).Should(BeFalse())
			Expect(probeObj.Status.Reason).Should(Equal("Status code: 500"))
		})
	})
})
