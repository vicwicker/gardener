// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package checker_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("HealthChecker", func() {
	var _ = Describe("health check", func() {
		var (
			ctx              = context.Background()
			fakeClient       client.Client
			fakeGardenClient client.Client
			fakeClock        = testclock.NewFakeClock(time.Now())

			condition gardencorev1beta1.Condition

			namespace = "shoot--foo--bar"
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			fakeClock = testclock.NewFakeClock(time.Now())
			condition = gardencorev1beta1.Condition{
				Type:               "test",
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			}
		})

		DescribeTable("#CheckManagedResource",
			func(conditions []gardencorev1beta1.Condition, upToDate bool, stepTime bool, conditionMatcher types.GomegaMatcher) {
				var (
					mr      = new(resourcesv1alpha1.ManagedResource)
					checker = NewHealthChecker(fakeClient, fakeClock)
				)

				if !upToDate {
					mr.Generation++
				}

				if stepTime {
					fakeClock.Step(5 * time.Minute)
				}

				mr.Status.Conditions = conditions

				exitCondition := checker.CheckManagedResource(condition, mr, &metav1.Duration{Duration: 5 * time.Minute})
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("no conditions",
				nil,
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg(gardencorev1beta1.ManagedResourceMissingConditionError, ""))),
			Entry("one true condition, one missing",
				[]gardencorev1beta1.Condition{
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg(gardencorev1beta1.ManagedResourceMissingConditionError, string(resourcesv1alpha1.ResourcesHealthy)))),
			Entry("multiple true conditions",
				[]gardencorev1beta1.Condition{
					{
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesProgressing,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				true,
				false,
				BeNil()),
			Entry("both progressing and healthy conditions are true for less than ManagedResourceProgressingThreshold",
				[]gardencorev1beta1.Condition{
					{
						Type:               resourcesv1alpha1.ResourcesProgressing,
						Status:             gardencorev1beta1.ConditionTrue,
						LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
					},
					{
						Type:   resourcesv1alpha1.ResourcesHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				false,
				BeNil()),
			Entry("both progressing and healthy conditions are true for more than ManagedResourceProgressingThreshold",
				[]gardencorev1beta1.Condition{
					{
						Type:               resourcesv1alpha1.ResourcesProgressing,
						Status:             gardencorev1beta1.ConditionTrue,
						LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
					},
					{
						Type:   resourcesv1alpha1.ResourcesHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				true,
				PointTo(beConditionWithFalseStatusReasonAndMsg(gardencorev1beta1.ManagedResourceProgressingRolloutStuck, "ManagedResource  is progressing for more than 5m0s"))),
			Entry("one false condition ResourcesApplied",
				[]gardencorev1beta1.Condition{
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionFalse,
					},
					{
						Type:   resourcesv1alpha1.ResourcesHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				false,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
			Entry("one false condition ResourcesHealthy",
				[]gardencorev1beta1.Condition{
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   resourcesv1alpha1.ResourcesHealthy,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				true,
				false,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
			Entry("multiple false conditions with reason & message & ResourcesApplied condition is not false",
				[]gardencorev1beta1.Condition{
					{
						Type:    resourcesv1alpha1.ResourcesHealthy,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "barFailed",
						Message: "bar is unhealthy",
					},
					{
						Type:    resourcesv1alpha1.ResourcesProgressing,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "fooFailed",
						Message: "foo is unhealthy",
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("barFailed", "bar is unhealthy"))),
			Entry("multiple false conditions with reason & message & ResourcesApplied condition is false",
				[]gardencorev1beta1.Condition{
					{
						Type:    resourcesv1alpha1.ResourcesHealthy,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "barFailed",
						Message: "bar is unhealthy",
					},
					{
						Type:    resourcesv1alpha1.ResourcesApplied,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "fooFailed",
						Message: "foo is unhealthy",
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("fooFailed", "foo is unhealthy"))),
			Entry("outdated managed resource",
				[]gardencorev1beta1.Condition{
					{
						Type:    resourcesv1alpha1.ResourcesApplied,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "fooFailed",
						Message: "foo is unhealthy",
					},
					{
						Type:    resourcesv1alpha1.ResourcesHealthy,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "barFailed",
						Message: "bar is unhealthy",
					},
				},
				false,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg(gardencorev1beta1.OutdatedStatusError, "outdated"))),
			Entry("unknown condition status with reason and message",
				[]gardencorev1beta1.Condition{
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:    resourcesv1alpha1.ResourcesHealthy,
						Status:  gardencorev1beta1.ConditionUnknown,
						Reason:  "Unknown",
						Message: "bar is unknown",
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("Unknown", "bar is unknown"))),
		)

		var (
			eventLoggerDepployment = newDeployment(namespace, v1beta1constants.DeploymentNameEventLogger, v1beta1constants.GardenRoleLogging, true)

			requiredLoggingControlPlaneDeployments = []*appsv1.Deployment{
				eventLoggerDepployment,
			}
		)

		DescribeTable("#CheckLoggingControlPlane",
			func(deployments []*appsv1.Deployment, eventLoggingEnabled bool, conditionMatcher types.GomegaMatcher) {
				for _, obj := range deployments {
					Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
				}

				checker := NewHealthChecker(fakeClient, fakeClock)
				exitCondition, err := checker.CheckLoggingControlPlane(ctx, namespace, eventLoggingEnabled, condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("all healthy",
				requiredLoggingControlPlaneDeployments,
				true,
				BeNil(),
			),
			Entry("required deployment is missing",
				nil,
				true,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			),
			Entry("deployment set unhealthy",
				[]*appsv1.Deployment{
					newDeployment(eventLoggerDepployment.Namespace, eventLoggerDepployment.Name, roleOf(eventLoggerDepployment), false),
				},
				true,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			),
			Entry("event logging is disabled in gardenlet config, omit deployment check",
				[]*appsv1.Deployment{},
				false,
				BeNil(),
			),
		)

		// CheckExtensionCondition
		DescribeTable("#CheckExtensionCondition - HealthCheckReport",
			func(healthCheckOutdatedThreshold *metav1.Duration, condition gardencorev1beta1.Condition, extensionsConditions []ExtensionCondition, expected types.GomegaMatcher) {
				checker := NewHealthChecker(fakeClient, fakeClock)
				updatedCondition := checker.CheckExtensionCondition(condition, extensionsConditions, healthCheckOutdatedThreshold)
				if expected == nil {
					Expect(updatedCondition).To(BeNil())
					return
				}
				Expect(updatedCondition).To(expected)
			},

			Entry("health check report is not outdated - threshold not configured in Gardenlet config",
				nil,
				gardencorev1beta1.Condition{Type: "type"},
				[]ExtensionCondition{
					{
						Condition: gardencorev1beta1.Condition{
							Type:   gardencorev1beta1.ShootControlPlaneHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
						LastHeartbeatTime: &metav1.MicroTime{Time: time.Now().Add(time.Second * -30)},
					},
				},
				BeNil(),
			),
			Entry("health check report is not outdated",
				// 2 minute threshold for outdated health check reports
				&metav1.Duration{Duration: time.Minute * 2},
				gardencorev1beta1.Condition{Type: "type"},
				[]ExtensionCondition{
					{
						Condition: gardencorev1beta1.Condition{
							Type:   gardencorev1beta1.ShootControlPlaneHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
						// health check result is only 30 seconds old so < than the staleExtensionHealthCheckThreshold
						LastHeartbeatTime: &metav1.MicroTime{Time: time.Now().Add(time.Second * -30)},
					},
				},
				BeNil(),
			),
			Entry("should determine that health check report is outdated - LastHeartbeatTime is nil",
				// 2 minute threshold for outdated health check reports
				&metav1.Duration{Duration: time.Minute * 2},
				gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.ShootControlPlaneHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				[]ExtensionCondition{
					{
						Condition: gardencorev1beta1.Condition{
							Type:   gardencorev1beta1.ShootControlPlaneHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
						ExtensionType:      "Worker",
						ExtensionName:      "worker-ubuntu",
						ExtensionNamespace: "shoot-namespace-in-seed",
					},
				},
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionUnknown)),
			),
			Entry("should determine that health check report is outdated",
				// 2 minute threshold for outdated health check reports
				&metav1.Duration{Duration: time.Minute * 2},
				gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.ShootControlPlaneHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				[]ExtensionCondition{
					{
						Condition: gardencorev1beta1.Condition{
							Type:   gardencorev1beta1.ShootControlPlaneHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
						ExtensionType:      "Worker",
						ExtensionName:      "worker-ubuntu",
						ExtensionNamespace: "shoot-namespace-in-seed",
						// health check result is already 3 minutes old
						LastHeartbeatTime: &metav1.MicroTime{Time: time.Now().Add(time.Minute * -3)},
					},
				},
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionUnknown)),
			),
			Entry("health check reports status progressing",
				nil,
				gardencorev1beta1.Condition{Type: "type"},
				[]ExtensionCondition{
					{
						ExtensionType: "Foo",
						Condition: gardencorev1beta1.Condition{
							Type:    gardencorev1beta1.ShootControlPlaneHealthy,
							Status:  gardencorev1beta1.ConditionProgressing,
							Reason:  "Bar",
							Message: "Baz",
						},
						LastHeartbeatTime: &metav1.MicroTime{Time: time.Now()},
					},
				},
				PointTo(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, "FooBar", "Baz")),
			),
			Entry("health check reports status false",
				nil,
				gardencorev1beta1.Condition{Type: "type"},
				[]ExtensionCondition{
					{
						ExtensionType: "Foo",
						Condition: gardencorev1beta1.Condition{
							Type:   gardencorev1beta1.ShootControlPlaneHealthy,
							Status: gardencorev1beta1.ConditionFalse,
						},
						LastHeartbeatTime: &metav1.MicroTime{Time: time.Now()},
					},
				},
				PointTo(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "FooUnhealthyReport", "failing health check")),
			),
			Entry("health check reports status unknown",
				nil,
				gardencorev1beta1.Condition{Type: "type"},
				[]ExtensionCondition{
					{
						ExtensionType: "Foo",
						Condition: gardencorev1beta1.Condition{
							Type:   gardencorev1beta1.ShootControlPlaneHealthy,
							Status: gardencorev1beta1.ConditionUnknown,
						},
						LastHeartbeatTime: &metav1.MicroTime{Time: time.Now()},
					},
				},
				PointTo(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "FooUnhealthyReport", "failing health check")),
			),
		)

		var (
			plutonoDeployment               = newDeployment(namespace, v1beta1constants.DeploymentNamePlutono, v1beta1constants.GardenRoleMonitoring, true)
			kubeStateMetricsShootDeployment = newDeployment(namespace, v1beta1constants.DeploymentNameKubeStateMetrics, v1beta1constants.GardenRoleMonitoring, true)

			requiredMonitoringControlPlaneDeployments = []*appsv1.Deployment{
				plutonoDeployment,
				kubeStateMetricsShootDeployment,
			}
		)

		DescribeTable("#CheckShootMonitoringControlPlane",
			func(deployments []*appsv1.Deployment, conditionMatcher types.GomegaMatcher) {
				for _, obj := range deployments {
					Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
				}

				checker := NewHealthChecker(fakeClient, fakeClock)

				exitCondition, err := checker.CheckMonitoringControlPlane(
					ctx,
					namespace,
					objectNameSet(requiredMonitoringControlPlaneDeployments),
					labels.SelectorFromSet(map[string]string{"gardener.cloud/role": "monitoring"}),
					condition,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("all healthy",
				requiredMonitoringControlPlaneDeployments,
				BeNil()),
			Entry("required deployment missing",
				[]*appsv1.Deployment{
					plutonoDeployment,
				},
				PointTo(beConditionWithMissingRequiredDeployment([]*appsv1.Deployment{kubeStateMetricsShootDeployment}))),
			Entry("deployment unhealthy",
				[]*appsv1.Deployment{
					newDeployment(plutonoDeployment.Namespace, plutonoDeployment.Name, roleOf(plutonoDeployment), false),
					kubeStateMetricsShootDeployment,
				},
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		)

		DescribeTable("#CheckControllerInstallation",
			func(conditions []gardencorev1beta1.Condition, upToDate bool, stepTime bool, conditionMatcher types.GomegaMatcher) {
				var checker = NewHealthChecker(fakeClient, fakeClock)

				controllerRegistration := &gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "foo",
					},
				}
				Expect(fakeGardenClient.Create(ctx, controllerRegistration)).To(Succeed())

				controllerInstallation := &gardencorev1beta1.ControllerInstallation{
					Spec: gardencorev1beta1.ControllerInstallationSpec{
						RegistrationRef: corev1.ObjectReference{
							Name:            controllerRegistration.Name,
							ResourceVersion: controllerRegistration.ResourceVersion,
						},
					},
				}

				if !upToDate {
					controllerInstallation.Spec.RegistrationRef.ResourceVersion = "0"
				}

				if stepTime {
					fakeClock.Step(5 * time.Minute)
				}

				controllerInstallation.Status.Conditions = conditions

				exitCondition, err := checker.CheckControllerInstallation(ctx, fakeGardenClient, condition, controllerInstallation, &metav1.Duration{Duration: 5 * time.Minute})
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("no conditions",
				nil,
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("MissingControllerInstallationCondition", ""))),
			Entry("one true condition, two missing",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("MissingControllerInstallationCondition", string(gardencorev1beta1.ControllerInstallationInstalled)))),
			Entry("multiple true conditions",
				[]gardencorev1beta1.Condition{
					{
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationProgressing,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				true,
				false,
				BeNil()),
			Entry("both progressing and healthy conditions are true for less than ControllerInstallationProgressingThreshold",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:               gardencorev1beta1.ControllerInstallationProgressing,
						Status:             gardencorev1beta1.ConditionTrue,
						LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				false,
				BeNil()),
			Entry("both progressing and healthy conditions are true for more than ControllerInstallationProgressingThreshold",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:               gardencorev1beta1.ControllerInstallationProgressing,
						Status:             gardencorev1beta1.ConditionTrue,
						LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				true,
				PointTo(beConditionWithFalseStatusReasonAndMsg("ProgressingRolloutStuck", "Seed : ControllerInstallation  is progressing for more than 5m0s"))),
			Entry("one false condition Valid",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionFalse,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				false,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
			Entry("one false condition Installed",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionFalse,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				false,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
			Entry("one false condition Healthy",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationHealthy,
						Status: gardencorev1beta1.ConditionFalse,
					},
				},
				true,
				false,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
			Entry("multiple false conditions with reason & message. Valid & Installed conditions are not false",
				[]gardencorev1beta1.Condition{
					{
						Type:    gardencorev1beta1.ControllerInstallationHealthy,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "barFailed",
						Message: "bar is unhealthy",
					},
					{
						Type:    gardencorev1beta1.ControllerInstallationProgressing,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "fooFailed",
						Message: "foo is unhealthy",
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("barFailed", "bar is unhealthy"))),
			Entry("multiple false conditions with reason & message & Installed condition is false",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:    gardencorev1beta1.ControllerInstallationHealthy,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "barFailed",
						Message: "bar is unhealthy",
					},
					{
						Type:    gardencorev1beta1.ControllerInstallationInstalled,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "fooFailed",
						Message: "foo is unhealthy",
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("fooFailed", "foo is unhealthy"))),
			Entry("outdated controller registration",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:    gardencorev1beta1.ControllerInstallationInstalled,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "fooFailed",
						Message: "foo is unhealthy",
					},
					{
						Type:    gardencorev1beta1.ControllerInstallationHealthy,
						Status:  gardencorev1beta1.ConditionFalse,
						Reason:  "barFailed",
						Message: "bar is unhealthy",
					},
				},
				false,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("OutdatedControllerRegistration", "outdated"))),
			Entry("unknown condition status with reason and message",
				[]gardencorev1beta1.Condition{
					{
						Type:   gardencorev1beta1.ControllerInstallationValid,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:   gardencorev1beta1.ControllerInstallationInstalled,
						Status: gardencorev1beta1.ConditionTrue,
					},
					{
						Type:    gardencorev1beta1.ControllerInstallationHealthy,
						Status:  gardencorev1beta1.ConditionUnknown,
						Reason:  "Unknown",
						Message: "bar is unknown",
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg("Unknown", "bar is unknown"))),
		)

		Describe("CheckManagedPrometheuses", func() {
			var (
				condition      = gardencorev1beta1.Condition{Type: "test"}
				filterTrueFunc = func(resourcesv1alpha1.ManagedResource) bool { return true }

				healthChecker                     *HealthChecker
				managedResources                  []resourcesv1alpha1.ManagedResource
				prometheus                        *monitoringv1.Prometheus
				testPrometheusHealthAlertsChecker health.PrometheusHealthAlertsChecker

				healthy   = func() (bool, error) { return false, nil }
				unhealthy = func() (bool, error) { return true, nil }
				erroring  = func() (bool, error) { return false, errors.New("test error") }
			)

			BeforeEach(func() {
				testPrometheusHealthAlertsChecker = func(_ context.Context, _ string, _ int) (bool, error) {
					msg := "testPrometheusHealthAlertsChecker should have been overridden"
					Fail(msg)
					return false, errors.New(msg)
				}

				healthChecker = NewHealthChecker(fakeClient, fakeClock, WithPrometheusHealthAlertsChecker(
					func(ctx context.Context, endpoint string, port int) (bool, error) {
						return testPrometheusHealthAlertsChecker(ctx, endpoint, port)
					}))

				managedResources = []resourcesv1alpha1.ManagedResource{{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-mr",
						Namespace: namespace,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						Resources: []resourcesv1alpha1.ObjectReference{
							{
								ObjectReference: corev1.ObjectReference{
									Kind:      monitoringv1.PrometheusesKind,
									Name:      "test-prometheus",
									Namespace: namespace,
								},
							},
						},
					},
				}}

				prometheus = &monitoringv1.Prometheus{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-prometheus",
						Namespace: namespace,
					},
					Spec: monitoringv1.PrometheusSpec{
						CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
							Replicas: ptr.To(int32(3)),
						},
					},
				}

				Expect(fakeClient.Create(ctx, prometheus)).To(Succeed())
			})

			It("should return nil when no prometheus instances are found", func() {
				managedResources[0].Status.Resources = []resourcesv1alpha1.ObjectReference{}

				result := healthChecker.CheckManagedPrometheuses(ctx, condition, managedResources, filterTrueFunc)
				Expect(result).To(BeNil())
			})

			It("should return nil when filter function returns false", func() {
				filterFalseFunc := func(resourcesv1alpha1.ManagedResource) bool { return false }

				result := healthChecker.CheckManagedPrometheuses(ctx, condition, managedResources, filterFalseFunc)
				Expect(result).To(BeNil())
			})

			It("should skip resources with ignore annotation", func() {
				managedResources[0].Annotations = map[string]string{
					resourcesv1alpha1.Ignore: "true",
				}

				result := healthChecker.CheckManagedPrometheuses(ctx, condition, managedResources, filterTrueFunc)
				Expect(result).To(BeNil())
			})

			It("should return error condition when prometheus resource is not found", func() {
				Expect(fakeClient.Delete(ctx, prometheus)).To(Succeed())

				result := healthChecker.CheckManagedPrometheuses(ctx, condition, managedResources, filterTrueFunc)
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Reason).To(Equal("PrometheusHealthAlertsError"))
				Expect(result.Message).To(Equal("Prometheus \"shoot--foo--bar/test-prometheus\" not found"))
			})

			It("should return error condition when client returns error", func() {
				errorClient := fakeclient.NewClientBuilder().
					WithScheme(kubernetes.SeedScheme).
					WithInterceptorFuncs(interceptor.Funcs{
						Get: func(ctx context.Context, client client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
							gvks, _, err := kubernetes.SeedScheme.ObjectKinds(obj)
							if err == nil && len(gvks) > 0 {
								if gvks[0].Kind == monitoringv1.PrometheusesKind {
									return errors.New("test error")
								}
							}
							return client.Get(ctx, key, obj, opts...)
						},
					}).
					Build()

				healthChecker := NewHealthChecker(errorClient, fakeClock)
				result := healthChecker.CheckManagedPrometheuses(ctx, condition, managedResources, filterTrueFunc)
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionUnknown))
				Expect(result.Reason).To(Equal("ConditionCheckError"))
				Expect(result.Message).To(Equal("failed checking Prometheus \"shoot--foo--bar/test-prometheus\": test error"))
			})

			It("should return error condition when health check returns error in at least a replica", func() {
				testPrometheusHealthAlertsChecker = func(_ context.Context, endpoint string, port int) (bool, error) {
					Expect(port).To(Equal(9090))
					switch endpoint {
					case "prometheus-test-prometheus-0.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return healthy()
					case "prometheus-test-prometheus-1.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return erroring()
					case "prometheus-test-prometheus-2.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return healthy()
					default:
						msg := "unexpected endpoint: " + endpoint
						Fail(msg)
						return false, errors.New(msg)
					}
				}

				result := healthChecker.CheckManagedPrometheuses(ctx, condition, managedResources, filterTrueFunc)
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Reason).To(Equal("PrometheusHealthAlertsError"))
				Expect(result.Message).To(Equal("Querying Prometheus \"shoot--foo--bar/test-prometheus\" for health alerts returned an error: test error"))
			})

			It("should return failing condition when health alerts are firing in at least a replica", func() {
				testPrometheusHealthAlertsChecker = func(_ context.Context, endpoint string, port int) (bool, error) {
					Expect(port).To(Equal(9090))
					switch endpoint {
					case "prometheus-test-prometheus-0.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return healthy()
					case "prometheus-test-prometheus-1.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return healthy()
					case "prometheus-test-prometheus-2.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return unhealthy()
					default:
						msg := "unexpected endpoint: " + endpoint
						Fail(msg)
						return false, errors.New(msg)
					}
				}

				result := healthChecker.CheckManagedPrometheuses(ctx, condition, managedResources, filterTrueFunc)
				Expect(result).NotTo(BeNil())
				Expect(result.Status).To(Equal(gardencorev1beta1.ConditionFalse))
				Expect(result.Reason).To(Equal("PrometheusHealthAlertsFiring"))
				Expect(result.Message).To(Equal("There are firing health alerts in Prometheus \"shoot--foo--bar/test-prometheus\". " +
					"Access Prometheus UI and check for firing ALERTS with type=\"health\"."))
			})

			It("should return nil when no health alerts are firing", func() {
				testPrometheusHealthAlertsChecker = func(_ context.Context, endpoint string, port int) (bool, error) {
					Expect(port).To(Equal(9090))
					switch endpoint {
					case "prometheus-test-prometheus-0.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return healthy()
					case "prometheus-test-prometheus-1.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return healthy()
					case "prometheus-test-prometheus-2.prometheus-operated.shoot--foo--bar.svc.cluster.local":
						return healthy()
					default:
						msg := "unexpected endpoint: " + endpoint
						Fail(msg)
						return false, errors.New(msg)
					}
				}

				result := healthChecker.CheckManagedPrometheuses(ctx, condition, managedResources, filterTrueFunc)
				Expect(result).To(BeNil())
			})
		})
	})
})

func beConditionWithStatusReasonAndMessage(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(WithStatus(status), WithReason(reason), WithMessage(message))
}

func beConditionWithStatus(status gardencorev1beta1.ConditionStatus) types.GomegaMatcher {
	return WithStatus(status)
}

func beConditionWithFalseStatusReasonAndMsg(reason, message string) types.GomegaMatcher {
	return And(WithStatus(gardencorev1beta1.ConditionFalse), WithReason(reason), WithMessage(message))
}

func beConditionWithMissingRequiredDeployment(deployments []*appsv1.Deployment) types.GomegaMatcher {
	var names = make([]string, 0, len(deployments))
	for _, deploy := range deployments {
		names = append(names, deploy.Name)
	}
	return And(WithStatus(gardencorev1beta1.ConditionFalse), WithMessage(fmt.Sprintf("%s", names)))
}

func roleOf(obj metav1.Object) string {
	return obj.GetLabels()[v1beta1constants.GardenRole]
}

func roleLabels(role string) map[string]string {
	return map[string]string{v1beta1constants.GardenRole: role}
}

func newDeployment(namespace, name, role string, healthy bool) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    roleLabels(role),
		},
	}
	if healthy {
		deployment.Status = appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		}}}
	}
	return deployment
}

func objectNameSet[o client.Object](objs []o) sets.Set[string] {
	names := sets.New[string]()

	for _, obj := range objs {
		names.Insert(obj.GetName())
	}

	return names
}
