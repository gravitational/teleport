package main

import (
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

	results, err := benchmark.Run(linear, "ls -l /", "ec2-3-15-147-120.us-east-2.compute.amazonaws.com", "ec2-user", "ec2-3-15-147-120.us-east-2.compute.amazonaws.com")
	if err != nil {
		fmt.Println(err)
	}

	for i, res := range results {
		fmt.Printf("Benchmark #%v\n", i)
		fmt.Printf("Duration: %v\n", res.Duration)
		fmt.Printf("Requests Originated: %v\n", res.RequestsOriginated)
		fmt.Printf("Requests Failed: %v\n", res.RequestsFailed)

	}

}
