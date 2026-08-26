/*
Copyright 2020 Gravitational, Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/benchmark"
)

func main() {
	linear := &benchmark.Linear{
		LowerBound:          10,
		UpperBound:          50,
		Step:                10,
		MinimumMeasurements: 1000,
		MinimumWindow:       30 * time.Second,
	}

	// Run Linear generator
	ctx := context.Background()
	results, err := benchmark.Run(
		ctx,
		linear,
		"host",
		"username",
		"teleport.example.com",
		benchmark.SSHBenchmark{
			Command: strings.Split("ls -l /", " "),
		},
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for i, res := range results {
		fmt.Printf("Benchmark #%v\n", i+1)
		fmt.Printf("Duration: %v\n", res.Duration)
		fmt.Printf("Requests Originated: %v\n", res.RequestsOriginated)
		fmt.Printf("Requests Failed: %v\n", res.RequestsFailed)
	}

	// Export latency profile
	responseHistogram := results[0].Histogram
	_, err = benchmark.ExportLatencyProfile(ctx, "profiles/", responseHistogram, 1, 1.0)
	if err != nil {
		fmt.Println(err)
	}
}
