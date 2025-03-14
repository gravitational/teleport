package main

import (
	"testing"
	"time"
)

func TestTickerRaceCondition(t *testing.T) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()

	// Goroutine that resets the ticker repeatedly
	go func() {
		for i := 0; i < 10000; i++ {
			ticker.Reset(time.Millisecond * 10) // **Write Operation**
		}
	}()

	// Goroutine that reads from ticker.C
	go func() {
		for i := 0; i < 10000; i++ {
			<-ticker.C // **Read Operation**
		}
	}()

	time.Sleep(time.Second) // Allow goroutines to run
}
