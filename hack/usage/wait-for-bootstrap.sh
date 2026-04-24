#!/usr/bin/env bash
#
# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

# This script waits until the garden Prometheus has evaluated the federation health
# check rules for the seed, then waits for all garden conditions to be healthy.
set -euo pipefail

TIMEOUT=${TIMEOUT:-900}
SLEEP_INTERVAL=${SLEEP_INTERVAL:-5}

echo "Waiting for garden Prometheus to load federation health check rules..."
retries=0
while [ "${retries}" -lt "${TIMEOUT}" ]; do
  result=$(kubectl exec -n garden prometheus-garden-0 -c prometheus -- wget -qO- 'http://localhost:9090/api/v1/rules?rule_name[]=healthcheck' 2>/dev/null) || true

  if echo "${result}" | jq -e '.data.groups[].rules[] | select(.labels.task == "federate:metering:absent" and .lastEvaluation != "")' &>/dev/null; then
    break
  fi

  retries=$((retries + SLEEP_INTERVAL))
  sleep "${SLEEP_INTERVAL}"
done

if [ "${retries}" -ge "${TIMEOUT}" ]; then
  echo "ERROR: Timed out waiting for federation health check rule after ${TIMEOUT} seconds."
  exit 1
fi

TIMEOUT=${TIMEOUT} ./hack/usage/wait-for.sh garden local VirtualGardenAPIServerAvailable RuntimeComponentsHealthy VirtualComponentsHealthy ObservabilityComponentsHealthy
