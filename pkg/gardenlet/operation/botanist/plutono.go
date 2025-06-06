// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/plutono"
	"github.com/gardener/gardener/pkg/component/shared"
)

// DefaultPlutono returns a deployer for Plutono.
func (b *Botanist) DefaultPlutono() (plutono.Interface, error) {
	dashboards := []string{plutono.ShootDashboardsPath, plutono.GardenAndShootDashboardsPath, plutono.CommonDashboardsPath}
	if b.Shoot.WantsVerticalPodAutoscaler {
		dashboards = append(dashboards, plutono.CommonVpaDashboardsPath)
	}
	if b.Shoot.IsWorkerless {
		dashboards = append(dashboards, plutono.ShootWorkerlessDashboardsPath)
	} else {
		dashboards = append(dashboards, plutono.ShootWorkerDashboardsPath)
		if b.ShootUsesDNS() {
			dashboards = append(dashboards, plutono.ShootWorkerIstioDashboardsPath)
		}
		if b.Shoot.VPNHighAvailabilityEnabled {
			dashboards = append(dashboards, plutono.ShootWorkerVpnSeedServerHaVpnDashboardsPath)
		} else {
			dashboards = append(dashboards, plutono.ShootWorkerVpnSeedServerEnvoyProxyDashboardsPath)
		}
	}

	return shared.NewPlutono(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		component.ClusterTypeShoot,
		b.Shoot.GetReplicas(1),
		"",
		b.ComputePlutonoHost(),
		v1beta1constants.PriorityClassNameShootControlPlane100,
		b.ShootUsesDNS(),
		b.Shoot.IsWorkerless,
		false,
		b.Shoot.VPNHighAvailabilityEnabled,
		b.Shoot.WantsVerticalPodAutoscaler,
		nil,
		plutono.ReadPaths(plutono.DashboardFS, dashboards...),
	)
}

// DeployPlutono deploys the plutono in the Seed cluster.
func (b *Botanist) DeployPlutono(ctx context.Context) error {
	// disable plutono if no observability components are needed
	if !b.WantsObservabilityComponents() {
		return b.Shoot.Components.ControlPlane.Plutono.Destroy(ctx)
	}

	if b.ControlPlaneWildcardCert != nil {
		b.Shoot.Components.ControlPlane.Plutono.SetWildcardCertName(ptr.To(b.ControlPlaneWildcardCert.GetName()))
	}

	return b.Shoot.Components.ControlPlane.Plutono.Deploy(ctx)
}
