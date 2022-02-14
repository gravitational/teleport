package main

import "log"

func main() {
	if err := run(); err != nil {
		log.Fatalf("FAILED: %s", err.Error())
	}
}

func run() error {
	return nil
}
