// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Components", func() {
	Describe("GetAPIServerSNIDomains", func() {
		It("should return the correct SNI domains", func() {
			domains := []string{"foo.bar", "bar.foo.bar", "foo.foo.bar", "foo.bar.foo.bar", "api.bar"}
			sni := operatorv1alpha1.SNI{
				DomainPatterns: []string{"api.bar", "*.foo.bar"},
			}

			Expect(GetAPIServerSNIDomains(domains, sni)).To(Equal([]string{"api.bar", "bar.foo.bar", "foo.foo.bar"}))
		})
	})

	Describe("garden dashboards", func() {
		It("should configure the correct dashboards", func() {
			garden := &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Settings: &operatorv1alpha1.Settings{},
					},
				},
			}

			fakeClient := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			reconciler := &Reconciler{
				RuntimeClientSet: fakekubernetes.NewClientSetBuilder().WithClient(fakeClient).Build(),
				GardenNamespace:  "garden",
			}
			plutono, err := reconciler.newPlutono(garden, nil, "", nil)
			if err != nil {
				panic(err)
			}

			var dashboards []string
			for d, _ := range plutono.Dashboards() {
				dashboards = append(dashboards, d)
			}

			Expect(dashboards).To(ConsistOf(
				[]string{
					"alerts-dashboard.json",
					"apiserver-admission-details.json",
					"apiserver-overview.json",
					"apiserver-request-details.json",
					"apiserver-request-duration-and-response-size.json",
					"apiserver-storage-details.json",
					"apiserver-watch-details.json",
					"container-runtime.json",
					"garden-alertmanager-dashboard.json",
					"garden-availability-dashboard.json",
					"garden-resource.json",
					"gardener-admission-controller-dashboard.json",
					"gardener-admission-controller-seedauthorizer-details.json",
					"gardener-controlplane.json",
					"kubernetes-pods-dashboard.json",
					"resource-usage-by-container.json",
					"seed-resource-usage-dashboard.json",
					"shoot-availability-dashboard.json",
					"shoot-details-dashboard.json",
					"shoot-sla-dashboard.json",
					"shoot-sli-dashboard.json",
					"virtual-garden-etcd-backup-dashboard.json",
					"virtual-garden-etcd-dashboard.json",
				},
			))
		})
	})
})
