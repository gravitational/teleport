package common

import (
	"context"
	"os"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/backend"
)

func onClone(ctx context.Context, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return trace.Wrap(err)
	}
	var config backend.CloneConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return trace.Wrap(err)
	}

	src, err := backend.New(ctx, config.Source.Type, config.Source.Params)
	if err != nil {
		return trace.Wrap(err, "failed to create source backend")
	}
	defer src.Close()

	dst, err := backend.New(ctx, config.Destination.Type, config.Destination.Params)
	if err != nil {
		return trace.Wrap(err, "failed to create destination backend")
	}
	defer dst.Close()

	if err := backend.Clone(ctx, src, dst, config.Parallel, config.Force); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
