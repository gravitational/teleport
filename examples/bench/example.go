package main

import (
	"context"
	"fmt"
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

	results, err := benchmark.Run(context.TODO(), linear, "ls -l /", "ec2-3-15-147-120.us-east-2.compute.amazonaws.com", "ec2-user", "ec2-3-15-147-120.us-east-2.compute.amazonaws.com")
	if err != nil {
		fmt.Println(err)
	}

	for i, res := range results {
		fmt.Printf("Benchmark #%v\n", i+1)
		fmt.Printf("Duration: %v\n", res.Duration)
		fmt.Printf("Requests Originated: %v\n", res.RequestsOriginated)
		fmt.Printf("Requests Failed: %v\n", res.RequestsFailed)

	}
}

/* 
 Example output: 
	Benchmark #1
	Duration: 1m40.040962769s
	Requests Originated: 1000
	Requests Failed: 0
	Benchmark #2
	Duration: 50.066970607s
	Requests Originated: 1000
	Requests Failed: 0
	Benchmark #3
	Duration: 33.353966907s
	Requests Originated: 1000
	Requests Failed: 0
	Benchmark #4
	Duration: 30.020290393s
	Requests Originated: 1200
	Requests Failed: 0
	Benchmark #5
	Duration: 37.399912317s
	Requests Originated: 1139
	Requests Failed: 55
*/