package main

import (
	"fmt"

	"github.com/gravitational/teleport/api"
)

func main() {
	fmt.Println(api.Version)
}
