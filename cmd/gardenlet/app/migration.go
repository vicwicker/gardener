// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/retry"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

func (g *garden) runMigrations(ctx context.Context, log logr.Logger, gardenClient client.Client) error {
	log.Info("Migrating deprecated failure-domain.beta.kubernetes.io labels to topology.kubernetes.io")
	if err := migrateDeprecatedTopologyLabels(ctx, log, g.mgr.GetClient()); err != nil {
		return err
	}

	if features.DefaultFeatureGate.Enabled(features.RemoveAPIServerProxyLegacyPort) {
		if err := verifyRemoveAPIServerProxyLegacyPortFeatureGate(ctx, gardenClient, g.config.SeedConfig.Name); err != nil {
			return err
		}
	}

	return nil
}

// TODO: Remove this function when Kubernetes 1.27 support gets dropped.
func migrateDeprecatedTopologyLabels(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	persistentVolumeList := &corev1.PersistentVolumeList{}
	if err := seedClient.List(ctx, persistentVolumeList); err != nil {
		return fmt.Errorf("failed listing persistent volumes for migrating deprecated topology labels: %w", err)
	}

	var taskFns []flow.TaskFn

	for _, pv := range persistentVolumeList.Items {
		persistentVolume := pv

		taskFns = append(taskFns, func(ctx context.Context) error {
			patch := client.MergeFrom(persistentVolume.DeepCopy())

			if persistentVolume.Spec.NodeAffinity == nil {
				// when PV is very old and has no node affinity, we just replace the topology labels
				if v, ok := persistentVolume.Labels[corev1.LabelFailureDomainBetaRegion]; ok {
					persistentVolume.Labels[corev1.LabelTopologyRegion] = v
				}
				if v, ok := persistentVolume.Labels[corev1.LabelFailureDomainBetaZone]; ok {
					persistentVolume.Labels[corev1.LabelTopologyZone] = v
				}
			} else if persistentVolume.Spec.NodeAffinity.Required != nil {
				// when PV has node affinity then we do not need the labels but just need to replace the topology keys
				// in the node selector term match expressions
				for i, term := range persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms {
					for j, expression := range term.MatchExpressions {
						if expression.Key == corev1.LabelFailureDomainBetaRegion {
							persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[j].Key = corev1.LabelTopologyRegion
						}

						if expression.Key == corev1.LabelFailureDomainBetaZone {
							persistentVolume.Spec.NodeAffinity.Required.NodeSelectorTerms[i].MatchExpressions[j].Key = corev1.LabelTopologyZone
						}
					}
				}
			}

			// either new topology labels were added above, or node affinity keys were adjusted
			// in both cases, the old, deprecated topology labels are no longer needed and can be removed
			delete(persistentVolume.Labels, corev1.LabelFailureDomainBetaRegion)
			delete(persistentVolume.Labels, corev1.LabelFailureDomainBetaZone)

			// prevent sending empty patches
			if data, err := patch.Data(&persistentVolume); err != nil {
				return fmt.Errorf("failed getting patch data for PV %s: %w", persistentVolume.Name, err)
			} else if string(data) == `{}` {
				return nil
			}

			log.Info("Migrating deprecated topology labels", "persistentVolumeName", persistentVolume.Name)
			return seedClient.Patch(ctx, &persistentVolume, patch)
		})
	}

	return flow.Parallel(taskFns...)(ctx)
}

// TODO(Wieneo): Remove this function when feature gate RemoveAPIServerProxyLegacyPort is removed
func verifyRemoveAPIServerProxyLegacyPortFeatureGate(ctx context.Context, gardenClient client.Client, seedName string) error {
	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList); err != nil {
		return err
	}

	for _, k := range shootList.Items {
		if specSeedName, statusSeedName := gardenerutils.GetShootSeedNames(&k); gardenerutils.GetResponsibleSeedName(specSeedName, statusSeedName) != seedName {
			continue
		}

		// we need to ignore shoots under the following conditions:
		// - it is workerless
		// - it is not yet picked up by gardenlet or still in phase "Creating"
		//
		// this is needed bcs. the constraint "ShootAPIServerProxyUsesHTTPProxy" is only set once the apiserver-proxy component is deployed to the shoot
		// this will never happen if the shoot is workerless or the component could still be missing, if the gardenlet is restarted during the creation of a shoot
		if v1beta1helper.IsWorkerless(&k) {
			continue
		}

		if k.Status.LastOperation == nil || ((k.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate || k.Status.LastOperation.Type == gardencorev1beta1.LastOperationTypeDelete) && k.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded) {
			continue
		}

		if cond := v1beta1helper.GetCondition(k.Status.Constraints, gardencorev1beta1.ShootAPIServerProxyUsesHTTPProxy); cond == nil || cond.Status != gardencorev1beta1.ConditionTrue {
			return errors.New("the `proxy` port on the istio ingress gateway cannot be removed until all api server proxies in all shoots on this seed have been reconfigured to use the `tls-tunnel` port instead, i.e., the `RemoveAPIServerProxyLegacyPort` feature gate can only be enabled once all shoots have the `APIServerProxyUsesHTTPProxy` constraint with status `true`")
		}
	}

	return nil
}

// syncBackupSecretRefAndCredentialsRef syncs the seed backup credentials when possible.
// TODO(vpnachev): Remove this function after v1.121.0 has been released.
func syncBackupSecretRefAndCredentialsRef(backup *gardencorev1beta1.Backup) {
	if backup == nil {
		return
	}

	emptySecretRef := corev1.SecretReference{}

	// secretRef is set and credentialsRef is not, sync both fields.
	if backup.SecretRef != emptySecretRef && backup.CredentialsRef == nil {
		backup.CredentialsRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  backup.SecretRef.Namespace,
			Name:       backup.SecretRef.Name,
		}

		return
	}

	// secretRef is unset and credentialsRef refer a secret, sync both fields.
	if backup.SecretRef == emptySecretRef && backup.CredentialsRef != nil &&
		backup.CredentialsRef.APIVersion == "v1" && backup.CredentialsRef.Kind == "Secret" {
		backup.SecretRef = corev1.SecretReference{
			Namespace: backup.CredentialsRef.Namespace,
			Name:      backup.CredentialsRef.Name,
		}

		return
	}

	// in all other cases we can do nothing:
	// - both fields are unset -> we have nothing to sync
	// - both fields are set -> let the validation check if they are correct
	// - credentialsRef refer to WorkloadIdentity -> secretRef should stay unset
}

func cleanupOldPrometheusFolders(ctx context.Context, log logr.Logger, seedClient client.Client) error {
	clusterList := &extensionsv1alpha1.ClusterList{}
	if err := seedClient.List(ctx, clusterList); err != nil {
		return fmt.Errorf("failed to list clusters for cleaning up old Prometheus folders: %w", err)
	}

	for _, cluster := range clusterList.Items {
		cleanupPod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: cluster.Name,
				Name:      "cleanup-obsolete-prometheus-folder",
			},
		}

		if err := seedClient.Delete(ctx, cleanupPod); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete cleanup Pod for Prometheus in cluster %s: %w", cluster.Name, err)
		}

		prometheus := &monitoringv1.Prometheus{}
		if err := seedClient.Get(ctx, client.ObjectKey{Namespace: cluster.Name, Name: "shoot"}, prometheus); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("failed to get Prometheus for cluster %s: %w", cluster.Name, err)
		}

		needsMigration := true
		for _, initContainer := range prometheus.Spec.InitContainers {
			if initContainer.Name == "cleanup-obsolete-folder" {
				needsMigration = false
				break
			}
		}

		if needsMigration {
			cleanupContainer := corev1.Container{
				Name:            "cleanup-obsolete-folder",
				Image:           *prometheus.Spec.Image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"rm", "-rf", "/prometheus/prometheus-"},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "prometheus-db",
					MountPath: "/prometheus",
				}},
			}

			if err := seedClient.Get(ctx, client.ObjectKey{Namespace: cluster.Name, Name: "prometheus-shoot"}, &corev1.Pod{}); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to get Pod for Prometheus in cluster %s: %w", cluster.Name, err)
			} else if prometheus.Spec.Replicas != nil && *prometheus.Spec.Replicas == 0 && err != nil {
				cleanupPod.Spec = corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers:    []corev1.Container{cleanupContainer},
					Volumes: []corev1.Volume{{
						Name: "prometheus-db",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "prometheus-db-prometheus-shoot-0",
							},
						},
					}},
				}

				if err := seedClient.Create(ctx, cleanupPod); client.IgnoreAlreadyExists(err) != nil {
					return fmt.Errorf("failed to create cleanup Pod for Prometheus in cluster %s: %w", cluster.Name, err)
				}

				return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
					pod := &corev1.Pod{}
					if err = seedClient.Get(ctx, client.ObjectKeyFromObject(cleanupPod), pod); err != nil {
						return retry.MinorError(fmt.Errorf("Error getting Pod: %v", err))
					}

					if pod.Status.Phase != corev1.PodSucceeded {
						return retry.MinorError(fmt.Errorf("Pod Status is not succeeded: %v", pod.Status.Phase))
					}

					return retry.Ok()
				})
			}

			prometheus.Spec.InitContainers = append(prometheus.Spec.InitContainers, cleanupContainer)
			patch := client.MergeFrom(prometheus.DeepCopy())
			if err := seedClient.Patch(ctx, prometheus, patch); err != nil {
				return fmt.Errorf("failed to patch Prometheus for cluster %s: %w", cluster.Name, err)
			}
		}

		if err := seedClient.Delete(ctx, cleanupPod); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete cleanup Pod for Prometheus in cluster %s: %w", cluster.Name, err)
		}
	}

	return nil
}
