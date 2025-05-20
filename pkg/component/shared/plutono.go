// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shared

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/plutono"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// NewPlutono returns a deployer for the plutono.
func NewPlutono(
	c client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	clusterType component.ClusterType,
	replicas int32,
	authSecretName, ingressHost, priorityClassName string,
	isGardenCluster bool,
	wildcardCertName *string,
	dashboards plutono.DashboardValues,
) (
	plutono.Interface,
	error,
) {
	plutonoImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePlutono)
	if err != nil {
		return nil, err
	}

	dashboardRefresherImage, err := imagevector.Containers().FindImage(imagevector.ContainerImageNamePlutonoDashboardRefresher)
	if err != nil {
		return nil, err
	}

	return plutono.New(
		c,
		namespace,
		secretsManager,
		plutono.Values{
			AuthSecretName:          authSecretName,
			ClusterType:             clusterType,
			Image:                   plutonoImage.String(),
			ImageDashboardRefresher: dashboardRefresherImage.String(),
			IngressHost:             ingressHost,
			IsGardenCluster:         isGardenCluster,
			PriorityClassName:       priorityClassName,
			Replicas:                replicas,
			WildcardCertName:        wildcardCertName,
			Dashboards:              dashboards,
		},
	), nil
}
