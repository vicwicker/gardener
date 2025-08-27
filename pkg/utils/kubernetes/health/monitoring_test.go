// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	"context"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	healthtest "github.com/gardener/gardener/test/utils/kubernetes/health"
)

var _ = Describe("Monitoring", func() {
	DescribeTable("CheckPrometheus",
		func(prometheus *monitoringv1.Prometheus, matcher types.GomegaMatcher) {
			err := health.CheckPrometheus(prometheus)
			Expect(err).To(matcher)
		},
		Entry("healthy", &monitoringv1.Prometheus{
			Spec:   monitoringv1.PrometheusSpec{CommonPrometheusFields: monitoringv1.CommonPrometheusFields{Replicas: ptr.To[int32](1)}},
			Status: monitoringv1.PrometheusStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, BeNil()),
		Entry("healthy with nil replicas", &monitoringv1.Prometheus{
			Status: monitoringv1.PrometheusStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, BeNil()),
		Entry("not observed at latest version", &monitoringv1.Prometheus{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
			Status:     monitoringv1.PrometheusStatus{Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, MatchError(ContainSubstring("observed generation outdated (0/1)"))),
		Entry("condition missing", &monitoringv1.Prometheus{}, MatchError(ContainSubstring(`condition "Available" is missing`))),
		Entry("condition False", &monitoringv1.Prometheus{
			Status: monitoringv1.PrometheusStatus{Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionFalse}}},
		}, MatchError(ContainSubstring(`condition "Available" has invalid status False (expected True)`))),
		Entry("not enough ready replicas", &monitoringv1.Prometheus{
			Spec:   monitoringv1.PrometheusSpec{CommonPrometheusFields: monitoringv1.CommonPrometheusFields{Replicas: ptr.To[int32](2)}},
			Status: monitoringv1.PrometheusStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, MatchError(ContainSubstring(`not enough available replicas (1/2)`))),
	)

	Describe("IsPrometheusProgressing", func() {
		var (
			prometheus *monitoringv1.Prometheus
		)

		BeforeEach(func() {
			prometheus = &monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: monitoringv1.PrometheusSpec{
					CommonPrometheusFields: monitoringv1.CommonPrometheusFields{Replicas: ptr.To[int32](3)},
				},
				Status: monitoringv1.PrometheusStatus{
					Conditions:      []monitoringv1.Condition{{Type: monitoringv1.Reconciled, Status: monitoringv1.ConditionTrue, ObservedGeneration: 42}},
					UpdatedReplicas: 3,
				},
			}
		})

		It("should return false if it is fully rolled out", func() {
			progressing, reason := health.IsPrometheusProgressing(prometheus)
			Expect(progressing).To(BeFalse())
			Expect(reason).To(Equal("Prometheus is fully rolled out"))
		})

		It("should return true if observedGeneration is outdated", func() {
			prometheus.Status.Conditions[0].ObservedGeneration--

			progressing, reason := health.IsPrometheusProgressing(prometheus)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("observed generation outdated (41/42)"))
		})

		It("should return true if replicas still need to be updated", func() {
			prometheus.Status.UpdatedReplicas--

			progressing, reason := health.IsPrometheusProgressing(prometheus)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("2 of 3 replica(s) have been updated"))
		})

		It("should return true if replica still needs to be updated (spec.replicas=null)", func() {
			prometheus.Spec.Replicas = nil
			prometheus.Status.UpdatedReplicas = 0

			progressing, reason := health.IsPrometheusProgressing(prometheus)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("0 of 1 replica(s) have been updated"))
		})
	})

	DescribeTable("CheckAlertmanager",
		func(alertManager *monitoringv1.Alertmanager, matcher types.GomegaMatcher) {
			err := health.CheckAlertmanager(alertManager)
			Expect(err).To(matcher)
		},
		Entry("healthy", &monitoringv1.Alertmanager{
			Spec:   monitoringv1.AlertmanagerSpec{Replicas: ptr.To[int32](1)},
			Status: monitoringv1.AlertmanagerStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, BeNil()),
		Entry("healthy with nil replicas", &monitoringv1.Alertmanager{
			Status: monitoringv1.AlertmanagerStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, BeNil()),
		Entry("not observed at latest version", &monitoringv1.Alertmanager{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
			Status:     monitoringv1.AlertmanagerStatus{Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, MatchError(ContainSubstring("observed generation outdated (0/1)"))),
		Entry("condition missing", &monitoringv1.Alertmanager{}, MatchError(ContainSubstring(`condition "Available" is missing`))),
		Entry("condition False", &monitoringv1.Alertmanager{
			Status: monitoringv1.AlertmanagerStatus{Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionFalse}}},
		}, MatchError(ContainSubstring(`condition "Available" has invalid status False (expected True)`))),
		Entry("not enough ready replicas", &monitoringv1.Alertmanager{
			Spec:   monitoringv1.AlertmanagerSpec{Replicas: ptr.To[int32](2)},
			Status: monitoringv1.AlertmanagerStatus{Replicas: 1, AvailableReplicas: 1, Conditions: []monitoringv1.Condition{{Type: monitoringv1.Available, Status: monitoringv1.ConditionTrue}}},
		}, MatchError(ContainSubstring(`not enough available replicas (1/2)`))),
	)

	Describe("IsAlertmanagerProgressing", func() {
		var (
			alertManager *monitoringv1.Alertmanager
		)

		BeforeEach(func() {
			alertManager = &monitoringv1.Alertmanager{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 42,
				},
				Spec: monitoringv1.AlertmanagerSpec{
					Replicas: ptr.To[int32](3),
				},
				Status: monitoringv1.AlertmanagerStatus{
					Conditions:      []monitoringv1.Condition{{Type: monitoringv1.Reconciled, Status: monitoringv1.ConditionTrue, ObservedGeneration: 42}},
					UpdatedReplicas: 3,
				},
			}
		})

		It("should return false if it is fully rolled out", func() {
			progressing, reason := health.IsAlertmanagerProgressing(alertManager)
			Expect(progressing).To(BeFalse())
			Expect(reason).To(Equal("Alertmanager is fully rolled out"))
		})

		It("should return true if observedGeneration is outdated", func() {
			alertManager.Status.Conditions[0].ObservedGeneration--

			progressing, reason := health.IsAlertmanagerProgressing(alertManager)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("observed generation outdated (41/42)"))
		})

		It("should return true if replicas still need to be updated", func() {
			alertManager.Status.UpdatedReplicas--

			progressing, reason := health.IsAlertmanagerProgressing(alertManager)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("2 of 3 replica(s) have been updated"))
		})

		It("should return true if replica still needs to be updated (spec.replicas=null)", func() {
			alertManager.Spec.Replicas = nil
			alertManager.Status.UpdatedReplicas = 0

			progressing, reason := health.IsAlertmanagerProgressing(alertManager)
			Expect(progressing).To(BeTrue())
			Expect(reason).To(Equal("0 of 1 replica(s) have been updated"))
		})
	})

	Describe("HasPrometheusHealthAlerts", func() {
		var server *healthtest.PrometheusServer

		BeforeEach(func() {
			server = healthtest.NewPrometheusServer()
		})

		AfterEach(func() {
			if server != nil {
				server.Close()
			}
		})

		It("should return true when alerts are present", func() {
			server.SetSuccessResponse("vector", "5")

			hasAlerts, err := health.HasPrometheusHealthAlerts(context.Background(), server.Endpoint(), server.Port())
			Expect(err).NotTo(HaveOccurred())
			Expect(hasAlerts).To(BeTrue())
		})

		It("should return false when no alerts are present", func() {
			server.SetSuccessResponse("vector", "0")

			hasAlerts, err := health.HasPrometheusHealthAlerts(context.Background(), server.Endpoint(), server.Port())
			Expect(err).NotTo(HaveOccurred())
			Expect(hasAlerts).To(BeFalse())
		})

		It("should handle prometheus error responses", func() {
			server.SetErrorResponse(http.StatusBadRequest, "invalid query")

			_, err := health.HasPrometheusHealthAlerts(context.Background(), server.Endpoint(), server.Port())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid query"))
		})

		It("should handle warnings in prometheus response", func() {
			server.SetWarningsResponse([]string{"some warning"})

			_, err := health.HasPrometheusHealthAlerts(context.Background(), server.Endpoint(), server.Port())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("query returned warnings"))
		})

		It("should handle unexpected result type", func() {
			server.SetMatrixResponse()

			_, err := health.HasPrometheusHealthAlerts(context.Background(), server.Endpoint(), server.Port())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("query returned an unexpected result type"))
		})

		It("should handle empty vector response", func() {
			server.SetEmptyVectorResponse()

			_, err := health.HasPrometheusHealthAlerts(context.Background(), server.Endpoint(), server.Port())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("query returned empty vector"))
		})
	})

	Describe("PrometheusEndpointFromHeadlessService", func() {

		It("should build endpoint for different replicas", func() {
			prometheus := &monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-prometheus",
					Namespace: "test-namespace",
				},
				Spec: monitoringv1.PrometheusSpec{
					CommonPrometheusFields: monitoringv1.CommonPrometheusFields{
						Replicas: ptr.To(int32(3)),
					},
				},
			}
			endpoint0, port0 := health.PrometheusEndpointFromHeadlessService(prometheus, 0)
			endpoint1, port1 := health.PrometheusEndpointFromHeadlessService(prometheus, 1)
			endpoint2, port2 := health.PrometheusEndpointFromHeadlessService(prometheus, 2)

			Expect(endpoint0).To(Equal("prometheus-test-prometheus-0.prometheus-operated.test-namespace.svc.cluster.local"))
			Expect(endpoint1).To(Equal("prometheus-test-prometheus-1.prometheus-operated.test-namespace.svc.cluster.local"))
			Expect(endpoint2).To(Equal("prometheus-test-prometheus-2.prometheus-operated.test-namespace.svc.cluster.local"))

			Expect(port0).To(Equal(9090))
			Expect(port1).To(Equal(9090))
			Expect(port2).To(Equal(9090))
		})
	})

	Describe("DefaultPrometheusEndpointBuilder", func() {

		It("should be set to PrometheusEndpointFromHeadlessService", func() {
			prometheus := &monitoringv1.Prometheus{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test-ns",
				},
			}

			defaultEndpoint, defaultPort := health.DefaultPrometheusEndpointBuilder(prometheus, 0)
			directEndpoint, directPort := health.PrometheusEndpointFromHeadlessService(prometheus, 0)

			Expect(defaultEndpoint).To(Equal(directEndpoint))
			Expect(defaultPort).To(Equal(directPort))
		})
	})
})
