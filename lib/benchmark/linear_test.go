/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package benchmark

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestGetBenchmark(t *testing.T) {
	initial := &Config{
		Rate:                0,
		MinimumWindow:       time.Second * 30,
		MinimumMeasurements: 1000,
	}

	linearConfig := Linear{
		LowerBound:          10,
		UpperBound:          50,
		Step:                10,
		MinimumMeasurements: 1000,
		MinimumWindow:       time.Second * 30,
		config:              initial,
	}
	expected := initial
	for _, rate := range []int{10, 20, 30, 40, 50} {
		expected.Rate = rate
		bm := linearConfig.GetBenchmark()
		require.Empty(t, cmp.Diff(expected, bm))
	}
	bm := linearConfig.GetBenchmark()
	require.Nil(t, bm)
}

func TestGetBenchmarkNotEvenMultiple(t *testing.T) {
	initial := &Config{
		Rate:                0,
		MinimumWindow:       time.Second * 30,
		MinimumMeasurements: 1000,
	}

	linearConfig := Linear{
		LowerBound:          10,
		UpperBound:          20,
		Step:                7,
		MinimumMeasurements: 1000,
		MinimumWindow:       time.Second * 30,
		config:              initial,
	}
	expected := initial
	bm := linearConfig.GetBenchmark()
	require.NotNil(t, bm)
	expected.Rate = 10
	require.Empty(t, cmp.Diff(expected, bm))

	bm = linearConfig.GetBenchmark()
	require.NotNil(t, bm)
	expected.Rate = 17
	require.Empty(t, cmp.Diff(expected, bm))

	bm = linearConfig.GetBenchmark()
	require.Nil(t, bm)
}

func TestValidateConfig(t *testing.T) {
	linearConfig := &Linear{
		LowerBound:          10,
		UpperBound:          20,
		Step:                7,
		MinimumMeasurements: 1000,
		MinimumWindow:       time.Second * 30,
		config:              nil,
	}
	err := validateConfig(linearConfig)
	require.NoError(t, err)

	linearConfig.MinimumWindow = time.Second * 0
	err = validateConfig(linearConfig)
	require.NoError(t, err)

	linearConfig.LowerBound = 21
	err = validateConfig(linearConfig)
	require.Error(t, err)
	linearConfig.LowerBound = 10

	linearConfig.MinimumMeasurements = 0
	err = validateConfig(linearConfig)
	require.Error(t, err)
}
