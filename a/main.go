package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	os.Args[0] = filepath.Join(filepath.Dir(os.Args[0]), "real", "link")
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	start := time.Now()
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Linker time: %s\n", time.Since(start))
}
