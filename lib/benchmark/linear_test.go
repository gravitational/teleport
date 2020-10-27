package benchmark

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	d, _ := time.ParseDuration("30s")
	initial := Config{
		Rate:                0,
		Command:             []string{"ls"},
		Interactive:         false,
		MinimumWindow:       d,
		MinimumMeasurements: 1000,
	}

	linearConfig := Linear{
		LowerBound:          10,
		UpperBound:          50,
		Step:                10,
		MinimumMeasurements: 1000,
		MinimumWindow:       d,
		config:              initial,
	}

	i := 0
	for i < linearConfig.UpperBound {
		res := linearConfig.Generate()
		require.True(t, res, "failed to generate generation %v", i/linearConfig.Step)
		bm, err := linearConfig.GetBenchmark()
		if err != nil {
			t.Errorf("failed to get current benchmark")
		}
		expected := initial
		expected.Rate = i + linearConfig.Step
		require.Empty(t, cmp.Diff(expected, bm))
		i += linearConfig.Step
	}

	res := linearConfig.Generate()
	require.False(t, res, "failed to generate generation %v", i/linearConfig.Step)
	_, err := linearConfig.GetBenchmark()
	if err == nil {
		t.Errorf("generating more benchmarks than expected")
	}
}

func TestGenerateNotEvenMultiple(t *testing.T) {
	d, _ := time.ParseDuration("30s")
	invalidBenchmarkD, _ := time.ParseDuration("0s")
	var tests = []struct {
		expected bool
		conf     Config
	}{
		{true, Config{
			Rate:                10,
			Command:             []string{"ls"},
			Interactive:         false,
			MinimumWindow:       d,
			MinimumMeasurements: 1000,
		}},
		{true, Config{
			Rate:                17,
			Command:             []string{"ls"},
			Interactive:         false,
			MinimumWindow:       d,
			MinimumMeasurements: 1000,
		}},
		{false, Config{
			Rate:                0,
			Command:             nil,
			Interactive:         false,
			MinimumWindow:       invalidBenchmarkD,
			MinimumMeasurements: 0,
		}},
	}

	initial := Config{
		Rate:                10,
		Command:             []string{"ls"},
		Interactive:         false,
		MinimumWindow:       d,
		MinimumMeasurements: 1000,
	}

	linearConfig := Linear{
		LowerBound:          10,
		UpperBound:          20,
		Step:                7,
		MinimumMeasurements: 1000,
		MinimumWindow:       d,
		config:              initial,
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			res := linearConfig.Generate()
			if res != tt.expected {
				t.Errorf("got %v, want %v", res, tt.conf.Rate)
			}
			fmt.Println(linearConfig.currentRPS)

			bm, err := linearConfig.GetBenchmark()
			if err != nil && bm.Rate != 0 {
				t.Errorf("failed to get current benchmark")
			}
			require.Empty(t, cmp.Diff(tt.conf, bm))
		})
	}
}

func TestGetBenchmark(t *testing.T) {
	d, _ := time.ParseDuration("30s")
	initial := Config{
		Rate:                0,
		Command:             []string{"ls"},
		Interactive:         false,
		MinimumWindow:       d,
		MinimumMeasurements: 1000,
	}
	linearConfig := Linear{
		LowerBound:          10,
		UpperBound:          20,
		Step:                10,
		MinimumMeasurements: 1000,
		MinimumWindow:       d,
		config:              initial,
	}
	// GetBenchmark with current generation
	res := linearConfig.Generate()
	initial.MinimumMeasurements = linearConfig.MinimumMeasurements
	require.Equal(t, res, true)
	conf, err := linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("expected benchmark, got error: %v", err)
	}
	require.Equal(t, conf.Rate, linearConfig.currentRPS)

	res = linearConfig.Generate()
	require.Equal(t, res, true)

	// GetBenchmark when Generate returns false
	res = linearConfig.Generate()
	require.Equal(t, res, false)

	if _, err = linearConfig.GetBenchmark(); err == nil {
		t.Errorf("There should be no current generations, expected error")
	}
}
