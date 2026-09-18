// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiserverfeatures "github.com/gardener/gardener/pkg/apiserver/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
)

func TestGardener(t *testing.T) {
	apiserverfeatures.RegisterFeatureGates()
	gardenletfeatures.RegisterFeatureGates()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Gardener Suite")
}
