package plutono

import "github.com/gardener/gardener/pkg/component"

func Dashboards(values Values) []string {
	if values.ClusterType == component.ClusterTypeSeed && !values.IsGardenCluster && !values.VPAEnabled {
		return []string{
			"alerts-dashboard.json",
			"client-go.json",
			"controller-details.json",
			"controllers.json",
			"extensions-dashboard.json",
			"fluent-bit-dashboard.json",
			"gardener-resource-usage-by-container.json",
			"gardener-resource-usage.json",
			"istio-control-plane-dashboard.json",
			"istio-ingress-gateway-dashboard.json",
			"istio-mesh-dashboard.json",
			"pod-logs.json",
			"seed-deployments-replicas.json",
			"seed-resource-usage.json",
			"shoot-control-plane-resource-usage-by-owner-container.json",
			"shoot-control-plane-resource-usage-overview.json",
			"shoot-control-plane-resource-usage.json",
			"shoot-operation-duration.json",
			"systemd-logs.json",
			"webhook-details.json",
			"webhooks.json",
		}
	}

	if values.ClusterType == component.ClusterTypeSeed && !values.IsGardenCluster && values.VPAEnabled {
		return []string{
			"alerts-dashboard.json",
			"client-go.json",
			"controller-details.json",
			"controllers.json",
			"extensions-dashboard.json",
			"fluent-bit-dashboard.json",
			"gardener-resource-usage-by-container.json",
			"gardener-resource-usage.json",
			"istio-control-plane-dashboard.json",
			"istio-ingress-gateway-dashboard.json",
			"istio-mesh-dashboard.json",
			"pod-logs.json",
			"seed-deployments-replicas.json",
			"seed-resource-usage.json",
			"shoot-control-plane-resource-usage-by-owner-container.json",
			"shoot-control-plane-resource-usage-overview.json",
			"shoot-control-plane-resource-usage.json",
			"shoot-operation-duration.json",
			"systemd-logs.json",
			"vpa-admission-controller.json",
			"vpa-dashboard.json",
			"vpa-recommender.json",
			"webhook-details.json",
			"webhooks.json",
		}
	}

	return nil
}
