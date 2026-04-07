//go:build !linux

package vnet

import (
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/trace"
)

func ServiceBuilder(cfg *Config, alpnUpgradeCache *internal.ALPNUpgradeCache) bot.ServiceBuilder {
	return bot.NewServiceBuilder(ServiceType, cfg.Name, func(bot.ServiceDependencies) (bot.Service, error) {
		return nil, trace.NotImplemented("%s service is only supported on linux", ServiceType)
	})
}
