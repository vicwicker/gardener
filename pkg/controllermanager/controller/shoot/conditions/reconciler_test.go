// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package conditions

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Reconciler", func() {
	Describe("#prefixOverlappingSeedConditions", func() {
		var shootConditionTypes []gardencorev1beta1.ConditionType

		BeforeEach(func() {
			shootConditionTypes = []gardencorev1beta1.ConditionType{
				gardencorev1beta1.ShootSystemComponentsHealthy,
				gardencorev1beta1.ShootObservabilityComponentsHealthy,
			}
		})

		It("should prefix only the seed conditions that overlap with shoot condition types", func() {
			seedConditions := []gardencorev1beta1.Condition{
				{Type: gardencorev1beta1.SeedSystemComponentsHealthy},
				{Type: gardencorev1beta1.SeedObservabilityComponentsHealthy},
			}

			result := prefixOverlappingSeedConditions(shootConditionTypes, seedConditions)

			Expect(result).To(HaveLen(2))
			Expect(result[0].Type).To(Equal(gardencorev1beta1.SeedSystemComponentsHealthy))
			Expect(result[1].Type).To(Equal(gardencorev1beta1.ConditionType("Seed" + string(gardencorev1beta1.SeedObservabilityComponentsHealthy))))
		})

		It("should not mutate the given seed conditions", func() {
			seedConditions := []gardencorev1beta1.Condition{
				{Type: gardencorev1beta1.SeedObservabilityComponentsHealthy},
			}

			_ = prefixOverlappingSeedConditions(shootConditionTypes, seedConditions)

			Expect(seedConditions[0].Type).To(Equal(gardencorev1beta1.SeedObservabilityComponentsHealthy))
		})
	})
})
