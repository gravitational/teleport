package common

import (
	"context"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/gravitational/teleport/lib/backend/clone"
	"github.com/gravitational/trace"
)

func onClone(ctx context.Context, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return trace.Wrap(err)
	}
	var config clone.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return trace.Wrap(err)
	}

	cloner, err := clone.New(ctx, config)
	if err != nil {
		return trace.Wrap(err)
	}
	defer cloner.Close()

	if err := cloner.Clone(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
