/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package embeddedtbot

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
)

// EmbeddedBot is an embedded tBot instance to renew the operator certificates.
type EmbeddedBot struct {
	cfg *config.BotConfig

	credential *config.UnstableClientCredentialOutput

	// mutex protects started, cancelCtx and errCh
	mutex     sync.Mutex
	started   bool
	cancelCtx func()
	errCh     chan error
}

// New creates a new EmbeddedBot from a BotConfig.
func New(botConfig *BotConfig) (*EmbeddedBot, error) {
	credential := &config.UnstableClientCredentialOutput{}

	cfg := (*config.BotConfig)(botConfig)
	cfg.Storage = &config.StorageConfig{Destination: &config.DestinationMemory{}}
	cfg.Outputs = []config.Output{credential}

	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bot := &EmbeddedBot{
		cfg:        cfg,
		credential: credential,
	}

	return bot, nil
}

// Preflight has two responsibilities:
// - connect to Teleport using tbot, get a certificate, validate that everything is set up properly (roles, bot, token, ...)
// - return the server features
// It allows us to fail fast and validate if something is broken before starting the manager.
func (b *EmbeddedBot) Preflight(ctx context.Context) (*proto.PingResponse, error) {
	b.cfg.Oneshot = true
	bot := tbot.New(b.cfg, slog.Default())
	err := bot.Run(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	teleportClient, err := b.buildClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pong, err := teleportClient.Ping(ctx)
	return &pong, trace.Wrap(err)
}

// start the bot and immediately returns.
// to be notified of the bot health there are two ways:
// - if the bot just started, waitForClient to make sure it obtained its first certificates
// - when the bot is running, call Start() that will wait for the bot to exit or the context being canceled.
func (b *EmbeddedBot) start(ctx context.Context) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.cfg.Oneshot = false
	bot := tbot.New(b.cfg, slog.Default())

	botCtx, cancel := context.WithCancel(ctx)
	b.cancelCtx = cancel
	b.errCh = make(chan error, 1)
	b.started = true

	go func() {
		err := bot.Run(botCtx)
		if err != nil {
			log.Errorf("bot exited with error: %s", err)
		} else {
			log.Infof("bot exited without error")
		}
		b.errCh <- trace.Wrap(err)
	}()
}

// Start is a lie, the bot is already started. DO NOT CALL Start if you want to run the bot, call StartAndWaitForClient.
// We mimick a legitimate Start() behavior by returning if the bot exists and propagating context cancellation.
func (b *EmbeddedBot) Start(ctx context.Context) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if !b.started {
		return errors.New("b.Start() is called but StartAndWaitForClient() has not been invoked yet, aborting")
	}

	// Start a goroutine that waits for the errorGroup and sends back the error
	select {
	case <-ctx.Done():
		// Context is canceled, we must stop the bot.
		b.cancelCtx()
		// Then we make sure the bot properly exited before returning.
		return trace.Wrap(<-b.errCh)
	case err := <-b.errCh:
		// Something happened to the bot, we must propagate the information to the manager.
		return trace.Wrap(err)
	}
}

func (b *EmbeddedBot) waitForClient(ctx context.Context, deadline time.Duration) (*client.Client, error) {
	waitCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	select {
	case <-waitCtx.Done():
		log.Warn("context canceled while waiting for the bot client")
		return nil, trace.Wrap(ctx.Err())
	case <-b.credential.Ready():
		log.Infof("credential ready")
	}

	c, err := b.buildClient(ctx)
	return c, trace.Wrap(err)

}

// StartAndWaitForClient starts the EmbeddedBot and waits for a client to be available.
// This is the proper way of starting the EmbeddedBot. It returns an error if the
// EmbeddedBot is not able to get a certificate before the deadline.
func (b *EmbeddedBot) StartAndWaitForClient(ctx context.Context, deadline time.Duration) (*client.Client, error) {
	b.start(ctx)
	c, err := b.waitForClient(ctx, deadline)
	return c, trace.Wrap(err)
}

// buildClient reads tbot's memory disttination, retrieves the certificates
// and builds a new Teleport client using those certs.
func (b *EmbeddedBot) buildClient(ctx context.Context) (*client.Client, error) {
	log.Infof("Building a new client to connect to %s", b.cfg.AuthServer)
	c, err := client.New(ctx, client.Config{
		Addrs:                    []string{b.cfg.AuthServer},
		Credentials:              []client.Credentials{b.credential},
		InsecureAddressDiscovery: b.cfg.Insecure,
	})
	return c, trace.Wrap(err)
}
