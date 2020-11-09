package benchmark

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

var (
	duration, _ = time.ParseDuration("30s")
)

func TestGenerate(t *testing.T) {
	initial := Config{
		Threads:             10,
		Rate:                0,
		Command:             []string{"ls"},
		Interactive:         false,
		MinimumWindow:       duration,
		MinimumMeasurements: 1000,
	}

	linearConfig := Linear{
		LowerBound:          10,
		UpperBound:          50,
		Step:                10,
		MinimumMeasurements: 1000,
		MinimumWindow:       duration,
		Threads:             10,
		config:              initial,
	}
	// First generation
	bm, err := linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("failed to get current benchmark")
	}
	expected := initial
	expected.Rate = 10

	assert.Empty(t, cmp.Diff(expected, bm))

	// Second generation
	bm, err = linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("failed to get current benchmark")
	}

	expected.Rate = 20
	assert.Empty(t, cmp.Diff(expected, bm))

	// Third generation
	bm, err = linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("failed to get current benchmark")
	}
	expected.Rate = 30
	assert.Empty(t, cmp.Diff(expected, bm))

	// Fourth generation
	bm, err = linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("failed to get current benchmark")
	}
	expected.Rate = 40
	assert.Empty(t, cmp.Diff(expected, bm))

	// Fifth generation
	bm, err = linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("failed to get current benchmark")
	}
	expected.Rate = 50
	assert.Empty(t, cmp.Diff(expected, bm))

	// Sixth generation, should return false
	_, err = linearConfig.GetBenchmark()
	if err == nil {
		t.Errorf("generating more benchmarks than expected")
	}
}

func TestGenerateNotEvenMultiple(t *testing.T) {

	initial := Config{
		Threads:             10,
		Rate:                0,
		Command:             []string{"ls"},
		Interactive:         false,
		MinimumWindow:       duration,
		MinimumMeasurements: 1000,
	}

	linearConfig := Linear{
		LowerBound:          10,
		UpperBound:          20,
		Step:                7,
		MinimumMeasurements: 1000,
		MinimumWindow:       duration,
		Threads:             10,
		config:              initial,
	}
	expected := initial
	bm, err := linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("failed to get current benchmark")
	}
	expected.Rate = 10
	assert.Empty(t, cmp.Diff(expected, bm))

	bm, err = linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("failed to get current benchmark")
	}
	expected.Rate = 17
	assert.Empty(t, cmp.Diff(expected, bm))

	_, err = linearConfig.GetBenchmark()
	if err == nil {
		t.Errorf("generating more benchmarks than expected")
	}
	expected.Rate = 17
	assert.Empty(t, cmp.Diff(expected, bm))
}

func TestGetBenchmark(t *testing.T) {
	initial := Config{
		Threads:             10,
		Rate:                0,
		Command:             []string{"ls"},
		Interactive:         false,
		MinimumWindow:       duration,
		MinimumMeasurements: 1000,
	}
	linearConfig := Linear{
		LowerBound:          10,
		UpperBound:          20,
		Step:                10,
		MinimumMeasurements: 1000,
		MinimumWindow:       duration,
		config:              initial,
	}

	// GetBenchmark with current generation

	initial.MinimumMeasurements = linearConfig.MinimumMeasurements

	conf, err := linearConfig.GetBenchmark()
	if err != nil {
		t.Errorf("expected benchmark, got error: %v", err)
	}
	assert.Equal(t, conf.Rate, linearConfig.currentRPS)
	linearConfig.GetBenchmark()
	// GetBenchmark when Generate returns false
	linearConfig.GetBenchmark()
	_, err = linearConfig.GetBenchmark()
	if err == nil {
		t.Errorf("There should be no current generations, expected error")
	}

}
