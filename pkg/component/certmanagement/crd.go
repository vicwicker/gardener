// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certmanagement

import (
	"context"
	_ "embed"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/utils/flow"
)

var (
	//go:embed assets/crd-cert.gardener.cloud_certificaterevocations.yaml
	crdRevocations string
	//go:embed assets/crd-cert.gardener.cloud_certificates.yaml
	crdCertificates string
	//go:embed assets/crd-cert.gardener.cloud_issuers.yaml
	crdIssuers string

	resources []string
)

func init() {
	resources = []string{crdRevocations, crdCertificates, crdIssuers}
}

type crdDeployer struct {
	applier kubernetes.Applier
}

// NewCRDs can be used to deploy the CRD definitions for the cert-management.
func NewCRDs(applier kubernetes.Applier) component.Deployer {
	return &crdDeployer{applier: applier}
}

func (c *crdDeployer) Deploy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return c.applier.ApplyManifest(ctx, kubernetes.NewManifestReader([]byte(r)), kubernetes.DefaultMergeFuncs)
		})
	}

	return flow.Parallel(fns...)(ctx)
}

func (c *crdDeployer) Destroy(ctx context.Context) error {
	var fns []flow.TaskFn

	for _, resource := range resources {
		r := resource
		fns = append(fns, func(ctx context.Context) error {
			return client.IgnoreNotFound(c.applier.DeleteManifest(ctx, kubernetes.NewManifestReader([]byte(r))))
		})
	}

	return flow.Parallel(fns...)(ctx)
}
