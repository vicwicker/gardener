// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cidr_test

import (
	"net"

	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/pkg/utils/validation/cidr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("cidr", func() {
	Describe("#cidr IPv4", func() {
		var (
			invalidGardenCIDR = "invalid_cidr"
			validGardenCIDR   = "10.0.0.0/8"
			path              = field.NewPath("foo")
		)

		Context("NewCIDR", func() {
			It("should return a non-nil value", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr).ToNot(BeNil())
			})

		})

		Context("GetCIDR", func() {
			It("should return a correct address", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.GetCIDR()).To(Equal(validGardenCIDR))
			})
		})

		Context("GetIPNet", func() {
			It("should return a correct IPNet", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				_, expected, _ := net.ParseCIDR(validGardenCIDR)

				actual := cdr.GetIPNet()

				Expect(actual).ToNot(BeNil())
				Expect(actual).To(Equal(expected))
			})

			It("should return an empty IPNet", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.GetIPNet()).To(BeNil())
			})
		})

		Context("GetFieldPath", func() {
			It("should return a correct FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				actual := cdr.GetFieldPath()

				Expect(actual).ToNot(BeNil())
				Expect(actual).To(Equal(path))
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, nil)

				Expect(cdr.GetFieldPath()).To(BeNil())
			})
		})

		Context("Parse", func() {
			It("should return a correct FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.Parse()).To(BeTrue())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.Parse()).To(BeFalse())
			})
		})

		Context("ValidateNotSubset", func() {
			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("2.2.2.2/32"), path)

				Expect(cdr.ValidateNotSubset(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateNotSubset(nil)).To(BeEmpty())
			})

			It("should ignore when parse error", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR(string("2.2.2.2/32"), path)

				Expect(cdr.ValidateNotSubset(other)).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badCIDR := string("10.0.0.1/32")
				badPath := field.NewPath("bad")
				other := NewCIDR(badCIDR, badPath)

				Expect(cdr.ValidateNotSubset(other)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`must not be a subset of "foo" ("10.0.0.0/8")`),
				}))
			})
		})

		Context("ValidateParse", func() {
			It("should parse without errors", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateParse()).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.ValidateParse()).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(invalidGardenCIDR),
					"Detail":   Equal(`invalid CIDR address: invalid_cidr`),
				}))
			})
		})

		Context("ValidateSubset", func() {
			It("should be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("10.0.0.1/32"), field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateSubset(nil)).To(BeEmpty())
			})

			It("should ignore parse errors", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR(string("10.0.0.1/32"), field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("10.0.0.1/32"), field.NewPath("bad"))

				Expect(other.ValidateSubset(cdr)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must be a subset of "bad" ("10.0.0.1/32")`),
				}))
			})

			It("superset subnet should not be a subset", func() {
				valid := NewCIDR(string("10.0.0.0/24"), field.NewPath("valid"))
				other := NewCIDR(validGardenCIDR, path)

				Expect(valid.ValidateSubset(other)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(other.GetIPNet().String()),
					"Detail":   Equal(`must be a subset of "valid" ("10.0.0.0/24")`),
				}))
			})
		})
	})
	Describe("#cidr IPv6", func() {
		var (
			invalidGardenCIDR = "invalid_cidr"
			validGardenCIDR   = "2001:0db8:85a3::/104"
			path              = field.NewPath("foo")
		)

		Context("NewCIDR", func() {
			It("should return a non-nil value", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr).ToNot(BeNil())
			})

		})

		Context("GetCIDR", func() {
			It("should return a correct address", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.GetCIDR()).To(Equal(validGardenCIDR))
			})
		})

		Context("GetIPNet", func() {
			It("should return a correct IPNet", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				_, expected, _ := net.ParseCIDR(validGardenCIDR)

				actual := cdr.GetIPNet()

				Expect(actual).ToNot(BeNil())
				Expect(actual).To(Equal(expected))
			})

			It("should return an empty IPNet", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.GetIPNet()).To(BeNil())
			})
		})

		Context("GetFieldPath", func() {
			It("should return a correct FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				actual := cdr.GetFieldPath()

				Expect(actual).ToNot(BeNil())
				Expect(actual).To(Equal(path))
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, nil)

				Expect(cdr.GetFieldPath()).To(BeNil())
			})
		})

		Context("Parse", func() {
			It("should return a correct FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.Parse()).To(BeTrue())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.Parse()).To(BeFalse())
			})
		})

		Context("ValidateNotSubset", func() {
			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("3001:0db8:85a3::1/128"), path)

				Expect(cdr.ValidateNotSubset(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateNotSubset(nil)).To(BeEmpty())
			})

			It("should ignore when parse error", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR(string("3001:0db8:85a3::1/128"), path)

				Expect(cdr.ValidateNotSubset(other)).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				badCIDR := string("2001:0db8:85a3::1/128")
				badPath := field.NewPath("bad")
				other := NewCIDR(badCIDR, badPath)

				Expect(cdr.ValidateNotSubset(other)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(badPath.String()),
					"BadValue": Equal(badCIDR),
					"Detail":   Equal(`must not be a subset of "foo" ("2001:0db8:85a3::/104")`),
				}))
			})
		})

		Context("ValidateParse", func() {
			It("should parse without errors", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateParse()).To(BeEmpty())
			})

			It("should return a nil FieldPath", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)

				Expect(cdr.ValidateParse()).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(invalidGardenCIDR),
					"Detail":   Equal(`invalid CIDR address: invalid_cidr`),
				}))
			})
		})

		Context("ValidateSubset", func() {
			It("should be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("2001:0db8:85a3::1/128"), field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should ignore nil values", func() {
				cdr := NewCIDR(validGardenCIDR, path)

				Expect(cdr.ValidateSubset(nil)).To(BeEmpty())
			})

			It("should ignore parse errors", func() {
				cdr := NewCIDR(invalidGardenCIDR, path)
				other := NewCIDR(string("2001:0db8:85a3::1/128"), field.NewPath("other"))

				Expect(cdr.ValidateSubset(other)).To(BeEmpty())
			})

			It("should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("2001:0db8:85a3::1/128"), field.NewPath("bad"))

				Expect(other.ValidateSubset(cdr)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must be a subset of "bad" ("2001:0db8:85a3::1/128")`),
				}))
			})

			It("superset subnet should not be a subset", func() {
				cdr := NewCIDR(validGardenCIDR, path)
				other := NewCIDR(string("2001:0db8:85a3::/128"), field.NewPath("bad"))

				Expect(other.ValidateSubset(cdr)).To(ConsistOfFields(Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal(path.String()),
					"BadValue": Equal(validGardenCIDR),
					"Detail":   Equal(`must be a subset of "bad" ("2001:0db8:85a3::/128")`),
				}))
			})

		})
	})
})
