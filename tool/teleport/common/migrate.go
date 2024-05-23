package common

import (
	"context"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/backend/migration"
	"github.com/gravitational/trace"
)

func runMigration(ctx context.Context, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return trace.Wrap(err)
	}
	var config migration.MigrationConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return trace.Wrap(err)
	}

	migration, err := migration.New(ctx, config)
	if err != nil {
		return trace.Wrap(err)
	}
	defer migration.Close()

	if err := migration.Run(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
