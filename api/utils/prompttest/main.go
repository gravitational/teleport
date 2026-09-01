// Compile with:
// > cd api
// > GOOS=windows GOARCH=amd64 go build -o prompttest.exe ./utils/prompttest

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/gravitational/teleport/api/utils/prompt"
)

func main() {
	enableFallback := flag.Bool("fallback", false, "Enable stdin terminal fallback")
	flag.Parse()

	// Enable callback if flag is given
	if *enableFallback {
		fmt.Printf("Enabling stdin terminal fallback\n")
		prompt.EnableStdinTerminalFallback()
	}

	pwd, err := prompt.Password(context.Background(), os.Stderr, prompt.Stdin(), "Enter your fake password; it will be printed after reading")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Your fake password: %s\n", pwd)
}
