package git

import (
	"context"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func UnshallowRepository(ctx context.Context, workspace string, deployKey []byte) error {
	log.Info("Configuring git")
	gitCfg, err := Configure(ctx, workspace, deployKey)
	if err != nil {
		return trace.Wrap(err, "failed configuring git")
	}
	defer gitCfg.Close()

	log.Info("Unshallowing repository")
	err = gitCfg.Do(ctx, "fetch", "--unshallow")
	if err != nil {
		return trace.Wrap(err, "unshallow failed")
	}

	return nil
}
