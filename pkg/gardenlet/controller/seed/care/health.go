// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	healthchecker "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// health contains information needed to execute health checks for a seed.
type health struct {
	seed          *gardencorev1beta1.Seed
	seedClient    client.Client
	clock         clock.Clock
	namespace     *string
	healthChecker *healthchecker.HealthChecker
}

// NewHealth creates a new Health instance with the given parameters.
func NewHealth(
	seed *gardencorev1beta1.Seed,
	seedClient client.Client,
	clock clock.Clock,
	namespace *string,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
) HealthCheck {
	return &health{
		seedClient:    seedClient,
		seed:          seed,
		clock:         clock,
		namespace:     namespace,
		healthChecker: healthchecker.NewHealthChecker(seedClient, clock, conditionThresholds, seed.Status.LastOperation),
	}
}

// Check conducts the health checks on all the given conditions.
func (h *health) Check(
	ctx context.Context,
	conditions SeedConditions,
) []gardencorev1beta1.Condition {
	var taskFns []flow.TaskFn

	managedResourcesGarden, err := h.listManagedResources(ctx, ptr.Deref(h.namespace, v1beta1constants.GardenNamespace))
	if err != nil {
		conditions.systemComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.systemComponentsHealthy, nil, err)
		conditions.observabilityComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.observabilityComponentsHealthy, nil, err)
		return conditions.ConvertToSlice()
	}

	managedResourcesIstio, err := h.listManagedResources(ctx, ptr.Deref(h.namespace, v1beta1constants.IstioSystemNamespace))
	if err != nil {
		conditions.systemComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.systemComponentsHealthy, nil, err)
	} else {
		taskFns = append(taskFns, func(ctx context.Context) error {
			newSystemComponentsCondition := h.checkSystemComponents(conditions.systemComponentsHealthy, append(managedResourcesGarden, managedResourcesIstio...))
			conditions.systemComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.systemComponentsHealthy, newSystemComponentsCondition, nil)
			return nil
		})
	}

	taskFns = append(taskFns, func(ctx context.Context) error {
		newObservabilityComponentsCondition := h.checkObservabilityComponents(ctx, conditions.observabilityComponentsHealthy, managedResourcesGarden)
		conditions.observabilityComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.observabilityComponentsHealthy, newObservabilityComponentsCondition, nil)
		return nil
	})

	_ = flow.Parallel(taskFns...)(ctx)

	return conditions.ConvertToSlice()
}

func (h *health) listManagedResources(ctx context.Context, namespace string) ([]resourcesv1alpha1.ManagedResource, error) {
	managedResourceList := &resourcesv1alpha1.ManagedResourceList{}
	if err := h.seedClient.List(ctx, managedResourceList, client.InNamespace(namespace)); err != nil {
		return nil, fmt.Errorf("failed listing ManagedResources in namespace %s: %w", namespace, err)
	}

	return managedResourceList.Items, nil
}

func (h *health) checkSystemComponents(condition gardencorev1beta1.Condition, managedResources []resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	if exitCondition := h.healthChecker.CheckManagedResources(condition, managedResources, func(managedResource resourcesv1alpha1.ManagedResource) bool {
		// Only the observability components have a specific condition type
		return managedResource.Spec.Class != nil && managedResource.Labels[v1beta1constants.LabelCareConditionType] != string(v1beta1constants.SeedObservabilityComponentsHealthy)
	}, nil); exitCondition != nil {
		return exitCondition
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy."))
}

func (h *health) checkObservabilityComponents(ctx context.Context, condition gardencorev1beta1.Condition, managedResources []resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	filterFn := func(managedResource resourcesv1alpha1.ManagedResource) bool {
		// TODO(vicwicker): Check at the end of PR of checking the class is necessary.
		return managedResource.Spec.Class != nil && managedResource.Labels[v1beta1constants.LabelCareConditionType] == string(v1beta1constants.SeedObservabilityComponentsHealthy)
	}

	if exitCondition := h.healthChecker.CheckManagedResources(condition, managedResources, filterFn, nil); exitCondition != nil {
		return exitCondition
	}

	for _, managedResource := range managedResources {
		if !filterFn(managedResource) && managedResource.Labels[managedresources.LabelKeyManagedBy] != managedresources.LabelValueManagedByGardenlet {
			continue
		}

		for _, resource := range managedResource.Status.Resources {
			if resource.Kind == monitoringv1.PrometheusesKind {
				prometheus := &monitoringv1.Prometheus{ObjectMeta: metav1.ObjectMeta{Namespace: resource.Namespace, Name: resource.Name}}
				if exitCondition, err := h.healthChecker.CheckHealthAlerts(ctx, condition, prometheus); err != nil {
					return ptr.To(v1beta1helper.NewConditionOrError(h.clock, condition, nil, fmt.Errorf("failed checking Prometheus %s/%s: %w", resource.Namespace, resource.Name, err)))
				} else if exitCondition != nil {
					return exitCondition
				}
			}
		}
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ObservabilityComponentsRunning", "All observability components are healthy."))
}

// SeedConditions contains all seed related conditions of the seed status subresource.
type SeedConditions struct {
	systemComponentsHealthy        gardencorev1beta1.Condition
	observabilityComponentsHealthy gardencorev1beta1.Condition
}

// ConvertToSlice returns the seed conditions as a slice.
func (s SeedConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
		s.systemComponentsHealthy,
		s.observabilityComponentsHealthy,
	}
}

// ConditionTypes returns all seed condition types.
func (s SeedConditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		s.systemComponentsHealthy.Type,
		s.observabilityComponentsHealthy.Type,
	}
}

// NewSeedConditions returns a new instance of SeedConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewSeedConditions(clock clock.Clock, status gardencorev1beta1.SeedStatus) SeedConditions {
	return SeedConditions{
		systemComponentsHealthy:        v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, gardencorev1beta1.SeedSystemComponentsHealthy),
		observabilityComponentsHealthy: v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, gardencorev1beta1.SeedObservabilityComponentsHealthy),
	}
}
