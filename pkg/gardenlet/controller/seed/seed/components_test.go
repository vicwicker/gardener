package seed

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	testclock "k8s.io/utils/clock/testing"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("seed dashboards", func() {
	It("should configure the correct dashboards", func() {
		gardenClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).Build()

		seedClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		seedClientSet := fakekubernetes.NewClientSetBuilder().WithClient(seedClient).Build()

		fakeDateAndTime, _ := time.Parse(time.DateTime, "2024-05-14 19:59:39")
		fakeClock := testclock.NewFakeClock(fakeDateAndTime)

		reconciler := &Reconciler{
			GardenClient:  gardenClient,
			SeedClientSet: seedClientSet,
			Clock:         fakeClock,
		}
		seed := &seedpkg.Seed{}
		seed.SetInfo(&gardencorev1beta1.Seed{
			Spec: gardencorev1beta1.SeedSpec{
				Settings: &gardencorev1beta1.SeedSettings{
					VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{
						Enabled: true,
					},
				},
			},
		})
		plutono, err := reconciler.newPlutono(seed, nil, nil, nil)
		if err != nil {
			panic(err)
		}
		dashboards := plutono.GetDashboards()

		// TODO(vicwicker): Test something smarter
		Expect(dashboards).To(HaveLen(24))
	})
})
