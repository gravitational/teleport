//go:build !debug

package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/gogo/protobuf/vanity/command"

	crdgen "github.com/gravitational/teleport/integrations/operator/crdgen"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

func main() {
	slog.SetDefault(slog.New(logutils.NewSlogTextHandler(os.Stderr,
		logutils.SlogTextHandlerConfig{
			Level: slog.LevelDebug,
		},
	)))

	req := command.Read()
	if err := crdgen.HandleJSONSchemaRequest(req); err != nil {
		slog.ErrorContext(context.Background(), "Failed to generate schema", "error", err)
		os.Exit(-1)
	}
}
