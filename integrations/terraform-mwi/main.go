package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/gravitational/teleport/integrations/terraform-mwi/provider"
)

func main() {
	ctx := context.Background()

	var (
		debug bool
	)
	flag.BoolVar(&debug, "debug", false, "enable debug logging")
	flag.Parse()

	err := providerserver.Serve(
		ctx,
		provider.New(),
		providerserver.ServeOpts{
			Address: "terraform.releases.teleport.dev/gravitational/teleport-mwi",
			Debug:   debug,
		},
	)
	if err != nil {
		// TODO: Use slog here?
		log.Fatal(err)
	}
}
