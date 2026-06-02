TITLE: Metering: recording-rule improvements

**How to categorize this PR?**
/area metering
/kind regression
/kind bug

**What this PR does / why we need it**:

This PR bundles three independent, individually revertible improvements to the metering recording rules. Per-commit details and rationale are in the respective commit messages; in short:

1. Project `garden_shoot_info` to only the labels that are relevant for metering, mitigating a regression caused by a recent label addition in gardener-metrics-exporter ([PR #145](https://github.com/gardener/gardener-metrics-exporter/pull/145)).
2. Unify and increase the `last_over_time()` window in the metering rules to 30m.
3. Drop a bogus `or sum_over_time` fallback in the cache / aggregate `:avg_over_time` recording rules. The same fix was previously applied to the corresponding `:meta:` rules in the garden Prometheus.

**Which issue(s) this PR fixes**:

Fixes #

**Special notes for your reviewer**:

- Each commit can be reviewed and reverted independently.
- All three changes have been verified offline against a real Gardener landscape's metering data.

**Release note**:

```bugfix operator
Improve robustness, accuracy, and resource consumption of the metering recording rules.
```
