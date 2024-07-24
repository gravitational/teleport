package common

import (
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/e/lib/pro"
	"github.com/gravitational/teleport/e/tool/teleport/process"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
)

type entProcessProvider struct {
}

func (p *entProcessProvider) NewTeleport(cfg *servicecfg.Config) (*service.TeleportProcess, error) {
	cfg.Auth.LicenseFile = "../../../fixtures/license-eub.pem"
	ps, err := process.NewTeleport(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	eps, ok := ps.(*pro.Process)
	if !ok {
		return nil, trace.BadParameter("expected teleport process, got %T", ps)
	}
	return eps.TeleportProcess, nil
}
