# Introduce `ObservabilityComponentsHealthy` condition to the `Seed`

**How to categorize this PR?**

/area monitoring
/kind enhancement

What this PR does / why we need it:

This PR adds a new condition `SeedObservabilityComponentsHealthy` to the `Seed`'s status, which indicates whether the observability components in the seed cluster are healthy.
Until now, the health of the observability components in the seed was contributed to the `SeedSystemComponentsHealthy` condition.
This was not ideal for two reasons:

1. The observability components health was not reported analogously across Gardener resources.
   The `Garden` and `Shoot` had their own `ObservabilityComponentsHealthy` condition, but the `Seed` did not.
2. Seeds block shoot operations if the `SeedSystemComponentsHealthy` condition is not true, which makes sense to protect shoots in a seed that cannot safely run them.
   Thus, if the infrastructure that deploys and takes care of shoots is not healthy, it is safer to stop that infrastructure from reconciling or deploying new shoots.
   However, it is not desired that failing observability health checks block such operations.
   For example, a certain Prometheus instance or exporter being unhealthy is irrelevant for shoot reconciliation.
   This has been particularly inconvenient since the introduction of the Prometheus health checks, which, due to their natural slowness given Prometheus scrape intervals, have potentially been disturbing shoot operations.
   End-to-end tests have suffered from this, since shoot operations can be delayed when the Prometheus health checks fail temporarily in the seed and slow down test execution, which risks a timeout.

The new condition is deliberately not part of the set that gates shoot operations, so a temporarily unhealthy observability stack no longer blocks shoot reconciliations or migrations.
It still contributes to managed seed rollout readiness and to the STATUS column printed by `kubectl get seed`, so operators keep a single place to see that a seed is fully healthy.

Unlike the existing `SeedSystemComponentsHealthy` condition, the new condition is not prefixed with `Seed`.
Instead, it reuses the same `ObservabilityComponentsHealthy` condition type that the `Garden` and `Shoot` resources use.
To avoid a collision when a `Shoot` is registered as a `Seed`, i.e., a managed seed, and the `Seed` conditions are copied onto the `Shoot`, the "Seed" prefix is added to any `Seed` condition that collides with a `Shoot` condition.
Thus, the condition is prefixed on the `Shoot`'s status while left unprefixed on the `Seed`.
Such prefixing is done generically, so that any future overlapping conditions are handled the same way.
We could eventually apply it to `SeedSystemComponentsHealthy` as well, aligning it with the existing `SystemComponentsHealthy` conditions, but that is out of scope for this PR.

**Which issue(s) this PR fixes:**

This PR addresses https://github.com/gardener/gardener/issues/15948.
Whether it fully eliminates the end-to-end test timeouts described in the issue will only be clear once it has merged and the end-to-end suites have run repeatedly across other PRs.

**Special notes for your reviewer:**

/cc @istvanballok @timebertt

**Release note:**

```other operator
The `Seed` resource now exposes a new `ObservabilityComponentsHealthy` condition that reports the health of the seed's observability components.
This health was previously folded into the `SeedSystemComponentsHealthy` condition, which was not analogous to the `ObservabilityComponentsHealthy` conditions on the `Garden` and `Shoot` resources.
Unlike `SeedSystemComponentsHealthy`, the new condition does not block shoot operations, so a temporarily unhealthy observability stack no longer prevents shoot reconciliations.
```
