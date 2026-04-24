// Adapted from github.com/oapi-codegen/runtime v1.4.0 (styleparam_test.go).
//
// Only the assertions covering the subset of behavior implemented in
// `styleparam.go` are kept: primitive scalars (and named aliases),
// `time.Time`, and `uuid.UUID` styled as "simple", "label", "matrix", or
// "form". Upstream cases for slices, maps, generic structs, byte-slice
// formatting, `types.Date`, and the "deepObject"/"spaceDelimited"/
// "pipeDelimited" styles were dropped, and `TestStyleParamUnsupported` below
// asserts the subset implementation returns an error when asked to handle
// them.
//
// Copyright 2019 DeepMap, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package oapiruntime

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestStyleParam(t *testing.T) {
	primitive := 5
	primitiveString := "123"
	primitiveStringWithReservedChar := "123;456"

	type AliasedTime time.Time
	ti, _ := time.Parse(time.RFC3339, "2020-01-01T22:00:00+02:00")
	timestamp := AliasedTime(ti)

	type AliasedUUID uuid.UUID
	aUUID := AliasedUUID(uuid.MustParse("baa07328-452e-40bd-aa2e-fa823ec13605"))

	// ---------------------------- Simple Style -------------------------------

	result, err := StyleParamWithLocation("simple", false, "id", ParamLocationQuery, primitive)
	assert.NoError(t, err)
	assert.EqualValues(t, "5", result)

	result, err = StyleParamWithLocation("simple", true, "id", ParamLocationQuery, primitive)
	assert.NoError(t, err)
	assert.EqualValues(t, "5", result)

	result, err = StyleParamWithLocation("simple", false, "id", ParamLocationQuery, timestamp)
	assert.NoError(t, err)
	assert.EqualValues(t, "2020-01-01T22%3A00%3A00%2B02%3A00", result)

	result, err = StyleParamWithLocation("simple", true, "id", ParamLocationQuery, timestamp)
	assert.NoError(t, err)
	assert.EqualValues(t, "2020-01-01T22%3A00%3A00%2B02%3A00", result)

	result, err = StyleParamWithLocation("simple", false, "id", ParamLocationQuery, &timestamp)
	assert.NoError(t, err)
	assert.EqualValues(t, "2020-01-01T22%3A00%3A00%2B02%3A00", result)

	result, err = StyleParamWithLocation("simple", true, "id", ParamLocationQuery, &timestamp)
	assert.NoError(t, err)
	assert.EqualValues(t, "2020-01-01T22%3A00%3A00%2B02%3A00", result)

	result, err = StyleParamWithLocation("simple", false, "id", ParamLocationQuery, aUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "baa07328-452e-40bd-aa2e-fa823ec13605", result)

	result, err = StyleParamWithLocation("simple", true, "id", ParamLocationQuery, aUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "baa07328-452e-40bd-aa2e-fa823ec13605", result)

	result, err = StyleParamWithLocation("simple", false, "id", ParamLocationQuery, &aUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "baa07328-452e-40bd-aa2e-fa823ec13605", result)

	result, err = StyleParamWithLocation("simple", true, "id", ParamLocationQuery, &aUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "baa07328-452e-40bd-aa2e-fa823ec13605", result)

	// ----------------------------- Label Style -------------------------------

	result, err = StyleParamWithLocation("label", false, "id", ParamLocationQuery, primitive)
	assert.NoError(t, err)
	assert.EqualValues(t, ".5", result)

	result, err = StyleParamWithLocation("label", true, "id", ParamLocationQuery, primitive)
	assert.NoError(t, err)
	assert.EqualValues(t, ".5", result)

	result, err = StyleParamWithLocation("label", false, "id", ParamLocationQuery, timestamp)
	assert.NoError(t, err)
	assert.EqualValues(t, ".2020-01-01T22%3A00%3A00%2B02%3A00", result)

	result, err = StyleParamWithLocation("label", false, "id", ParamLocationQuery, aUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, ".baa07328-452e-40bd-aa2e-fa823ec13605", result)

	// ----------------------------- Matrix Style ------------------------------

	result, err = StyleParamWithLocation("matrix", false, "id", ParamLocationQuery, primitive)
	assert.NoError(t, err)
	assert.EqualValues(t, ";id=5", result)

	result, err = StyleParamWithLocation("matrix", true, "id", ParamLocationQuery, primitive)
	assert.NoError(t, err)
	assert.EqualValues(t, ";id=5", result)

	result, err = StyleParamWithLocation("matrix", false, "id", ParamLocationQuery, timestamp)
	assert.NoError(t, err)
	assert.EqualValues(t, ";id=2020-01-01T22%3A00%3A00%2B02%3A00", result)

	result, err = StyleParamWithLocation("matrix", false, "id", ParamLocationQuery, aUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, ";id=baa07328-452e-40bd-aa2e-fa823ec13605", result)

	// ------------------------------ Form Style -------------------------------

	result, err = StyleParamWithLocation("form", false, "id", ParamLocationQuery, primitive)
	assert.NoError(t, err)
	assert.EqualValues(t, "id=5", result)

	result, err = StyleParamWithLocation("form", false, "id", ParamLocationQuery, primitiveString)
	assert.NoError(t, err)
	assert.EqualValues(t, "id=123", result)

	result, err = StyleParamWithLocation("form", false, "id", ParamLocationQuery, primitiveStringWithReservedChar)
	assert.NoError(t, err)
	assert.EqualValues(t, "id=123%3B456", result)

	result, err = StyleParamWithLocation("form", false, "id", ParamLocationQuery, timestamp)
	assert.NoError(t, err)
	assert.EqualValues(t, "id=2020-01-01T22%3A00%3A00%2B02%3A00", result)

	result, err = StyleParamWithLocation("form", false, "id", ParamLocationQuery, &timestamp)
	assert.NoError(t, err)
	assert.EqualValues(t, "id=2020-01-01T22%3A00%3A00%2B02%3A00", result)

	result, err = StyleParamWithLocation("form", false, "id", ParamLocationQuery, aUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "id=baa07328-452e-40bd-aa2e-fa823ec13605", result)

	result, err = StyleParamWithLocation("form", false, "id", ParamLocationQuery, &aUUID)
	assert.NoError(t, err)
	assert.EqualValues(t, "id=baa07328-452e-40bd-aa2e-fa823ec13605", result)

	// -------------------------- Misc / type aliases --------------------------

	type StrType string
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, StrType("test"))
	assert.NoError(t, err)
	assert.EqualValues(t, "test", result)

	type IntType int32
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, IntType(7))
	assert.NoError(t, err)
	assert.EqualValues(t, "7", result)

	type UintType uint
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, UintType(9))
	assert.NoError(t, err)
	assert.EqualValues(t, "9", result)

	type Uint8Type uint8
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, Uint8Type(9))
	assert.NoError(t, err)
	assert.EqualValues(t, "9", result)

	type Uint16Type uint16
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, Uint16Type(9))
	assert.NoError(t, err)
	assert.EqualValues(t, "9", result)

	type Uint32Type uint32
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, Uint32Type(9))
	assert.NoError(t, err)
	assert.EqualValues(t, "9", result)

	type Uint64Type uint64
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, Uint64Type(9))
	assert.NoError(t, err)
	assert.EqualValues(t, "9", result)

	type FloatType64 float64
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, FloatType64(7.5))
	assert.NoError(t, err)
	assert.EqualValues(t, "7.5", result)

	type FloatType32 float32
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, FloatType32(1.05))
	assert.NoError(t, err)
	assert.EqualValues(t, "1.05", result)

	uuidValue := uuid.MustParse("c2d07ba4-5106-4eab-bcad-0bd6068dcb1a")
	result, err = StyleParamWithLocation("simple", false, "foo", ParamLocationQuery, uuidValue)
	assert.NoError(t, err)
	assert.EqualValues(t, "c2d07ba4-5106-4eab-bcad-0bd6068dcb1a", result)

	// Plain time.Time / uuid.UUID (no named alias) hit the TextMarshaler branch
	// slightly differently than the aliased variants above; cover both.
	timeVal := time.Date(1996, time.March, 19, 0, 0, 0, 0, time.UTC)
	result, err = StyleParamWithLocation("simple", false, "id", ParamLocationQuery, timeVal)
	assert.NoError(t, err)
	assert.EqualValues(t, "1996-03-19T00%3A00%3A00Z", result)

	uuidD := uuid.MustParse("972beb41-e5ea-4b31-a79a-96f4999d8769")
	result, err = StyleParamWithLocation("simple", false, "id", ParamLocationQuery, uuidD)
	assert.NoError(t, err)
	assert.EqualValues(t, "972beb41-e5ea-4b31-a79a-96f4999d8769", result)
}

// TestStyleParamAllowReserved covers the primitive-value cases from upstream's
// allowReserved tests. Array cases were dropped since we don't support slices.
func TestStyleParamAllowReserved(t *testing.T) {
	opts := func(allowReserved bool) StyleParamOptions {
		return StyleParamOptions{
			ParamLocation: ParamLocationQuery,
			AllowReserved: allowReserved,
		}
	}

	t.Run("primitive with reserved chars", func(t *testing.T) {
		value := "List(79988552,27056405)"

		result, err := StyleParamWithOptions("form", false, "ids", value, opts(false))
		assert.NoError(t, err)
		assert.EqualValues(t, "ids=List%2879988552%2C27056405%29", result, "reserved chars should be encoded when allowReserved=false")

		result, err = StyleParamWithOptions("form", false, "ids", value, opts(true))
		assert.NoError(t, err)
		assert.EqualValues(t, "ids=List(79988552,27056405)", result, "reserved chars should be preserved when allowReserved=true")
	})

	t.Run("primitive with colons and slashes", func(t *testing.T) {
		value := "2020-01-01T22:00:00+02:00"

		result, err := StyleParamWithOptions("form", false, "ts", value, opts(false))
		assert.NoError(t, err)
		assert.EqualValues(t, "ts=2020-01-01T22%3A00%3A00%2B02%3A00", result)

		result, err = StyleParamWithOptions("form", false, "ts", value, opts(true))
		assert.NoError(t, err)
		assert.EqualValues(t, "ts=2020-01-01T22:00:00+02:00", result)
	})

	t.Run("spaces still encoded with allowReserved", func(t *testing.T) {
		value := "hello world"

		result, err := StyleParamWithOptions("form", false, "q", value, opts(true))
		assert.NoError(t, err)
		assert.EqualValues(t, "q=hello%20world", result, "spaces should still be encoded even with allowReserved=true")
	})

	t.Run("allowReserved has no effect on non-query locations", func(t *testing.T) {
		value := "a;b"

		result, err := StyleParamWithOptions("simple", false, "id", value, StyleParamOptions{
			ParamLocation: ParamLocationPath,
			AllowReserved: true,
		})
		assert.NoError(t, err)
		assert.EqualValues(t, "a%3Bb", result, "path params should always encode reserved chars")
	})

	t.Run("zero value preserves existing behavior", func(t *testing.T) {
		value := "123;456"

		result, err := StyleParamWithOptions("form", false, "id", value, StyleParamOptions{
			ParamLocation: ParamLocationQuery,
		})
		assert.NoError(t, err)
		assert.EqualValues(t, "id=123%3B456", result)
	})
}

// TestStyleParamNameEncoding covers the primitive-value cases from upstream's
// name-encoding tests.
func TestStyleParamNameEncoding(t *testing.T) {
	t.Run("brackets in primitive param name", func(t *testing.T) {
		result, err := StyleParamWithOptions("form", false, "filter[name]", "foo", StyleParamOptions{ParamLocation: ParamLocationQuery})
		assert.NoError(t, err)
		assert.EqualValues(t, "filter%5Bname%5D=foo", result)
	})

	t.Run("simple alphanumeric name unchanged", func(t *testing.T) {
		result, err := StyleParamWithOptions("form", false, "color", "blue", StyleParamOptions{ParamLocation: ParamLocationQuery})
		assert.NoError(t, err)
		assert.EqualValues(t, "color=blue", result)
	})

	t.Run("path param name not encoded", func(t *testing.T) {
		result, err := StyleParamWithOptions("matrix", false, "id", "5", StyleParamOptions{
			ParamLocation: ParamLocationPath,
		})
		assert.NoError(t, err)
		assert.EqualValues(t, ";id=5", result)
	})
}

// TestStyleParamUnsupported pins down the behavior of the subset: anything
// that was valid upstream but isn't implemented here must return an error
// rather than silently producing a malformed URL. If a future regeneration of
// the access-graph client starts emitting one of these shapes, the failing
// test is the signal to port over the matching upstream code.
func TestStyleParamUnsupported(t *testing.T) {
	opts := StyleParamOptions{ParamLocation: ParamLocationQuery}

	t.Run("slice", func(t *testing.T) {
		_, err := StyleParamWithOptions("form", false, "id", []int{1, 2, 3}, opts)
		assert.Error(t, err)
	})

	t.Run("byte slice with format byte", func(t *testing.T) {
		_, err := StyleParamWithOptions("form", false, "data", []byte("test"), StyleParamOptions{
			ParamLocation: ParamLocationQuery,
			Type:          "string",
			Format:        "byte",
		})
		assert.Error(t, err)
	})

	t.Run("map", func(t *testing.T) {
		_, err := StyleParamWithOptions("form", true, "id", map[string]string{"k": "v"}, opts)
		assert.Error(t, err)
	})

	t.Run("generic struct", func(t *testing.T) {
		type Obj struct {
			Name string `json:"name"`
		}
		_, err := StyleParamWithOptions("form", true, "id", Obj{Name: "foo"}, opts)
		assert.Error(t, err)
	})

	t.Run("deepObject style", func(t *testing.T) {
		_, err := StyleParamWithOptions("deepObject", true, "id", "scalar", opts)
		assert.Error(t, err)
	})

	t.Run("spaceDelimited style", func(t *testing.T) {
		_, err := StyleParamWithOptions("spaceDelimited", false, "id", "scalar", opts)
		assert.Error(t, err)
	})

	t.Run("pipeDelimited style", func(t *testing.T) {
		_, err := StyleParamWithOptions("pipeDelimited", false, "id", "scalar", opts)
		assert.Error(t, err)
	})

	t.Run("nil pointer", func(t *testing.T) {
		var ptr *int
		_, err := StyleParamWithOptions("form", false, "id", ptr, opts)
		assert.Error(t, err)
	})
}
