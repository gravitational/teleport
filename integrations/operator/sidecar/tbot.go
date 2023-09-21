/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package sidecar

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/kubernetestoken"
	"github.com/gravitational/teleport/lib/tbot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	DefaultCertificateTTL  = 2 * time.Hour
	DefaultRenewalInterval = 30 * time.Minute
)

// onboardingConfigGetter is a function that returns tbot's onboarding config.
type onboardingConfigGetter func(ctx context.Context, options Options, client auth.ClientI) (*config.OnboardingConfig, error)

// ClientAccessor returns a working teleport api client when invoked.
// Client users should always call this function on a regular basis to ensure certs are always valid.
type ClientAccessor func(ctx context.Context) (*client.Client, error)

// Bot is a wrapper around an embedded tbot.
// It implements sigs.k8s.io/controller-runtime/manager.Runnable and
// sigs.k8s.io/controller-runtime/manager.LeaderElectionRunnable so it can be added to a controllerruntime.Manager.
type Bot struct {
	cfg                 *config.BotConfig
	running             bool
	rootClient          auth.ClientI
	opts                Options
	getOnboardingConfig onboardingConfigGetter
}

func (b *Bot) initializeConfig(ctx context.Context) {
	// Initialize the memory stores. They contain identities renewed by the bot
	// We're reading certs directly from them
	rootMemoryStore := &config.DestinationMemory{}
	destMemoryStore := &config.DestinationMemory{}

	// Initialize tbot config
	b.cfg = &config.BotConfig{
		Onboarding: config.OnboardingConfig{
			TokenValue: "",         // Field should be populated later, before running
			CAPins:     []string{}, // Field should be populated later, before running
			JoinMethod: "",
		},
		Storage: &config.StorageConfig{
			Destination: rootMemoryStore,
		},
		Outputs: []config.Output{
			&config.IdentityOutput{
				Destination: destMemoryStore,
			},
		},

		Debug:           false,
		AuthServer:      b.opts.Addr,
		CertificateTTL:  DefaultCertificateTTL,
		RenewalInterval: DefaultRenewalInterval,
		Oneshot:         false,
	}
	// We do our own init because config's "CheckAndSetDefaults" is too linked with tbot logic and invokes
	// `addRequiredConfigs` on each Storage Destination
	_ = rootMemoryStore.CheckAndSetDefaults()
	_ = destMemoryStore.CheckAndSetDefaults()

	for _, artifact := range identity.GetArtifacts() {
		_ = destMemoryStore.Write(ctx, artifact.Key, []byte{})
		_ = rootMemoryStore.Write(ctx, artifact.Key, []byte{})
	}

}

func (b *Bot) GetClient(ctx context.Context) (*client.Client, error) {
	if !b.running {
		return nil, trace.Errorf("bot not started yet")
	}
	// If the bot has not joined the cluster yet or not generated client certs we bail out
	// This is either temporary or the bot is dead and the manager will shut down everything.
	storageDestination := b.cfg.Storage.Destination
	if botCert, err := storageDestination.Read(ctx, identity.TLSCertKey); err != nil || len(botCert) == 0 {
		return nil, trace.Retry(err, "bot cert not yet present")
	}
	if cert, err := b.cfg.Outputs[0].GetDestination().Read(ctx, identity.TLSCertKey); err != nil || len(cert) == 0 {
		return nil, trace.Retry(err, "cert not yet present")
	}

	// Hack to be able to reuse LoadIdentity functions from tbot
	// LoadIdentity expects to have all the artifacts required for a bot
	// We loop over missing artifacts and are loading them from the bot storage to the destination
	for _, artifact := range identity.GetArtifacts() {
		if artifact.Kind == identity.KindBotInternal {
			value, err := storageDestination.Read(ctx, artifact.Key)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if err := b.cfg.Outputs[0].GetDestination().Write(ctx, artifact.Key, value); err != nil {
				return nil, trace.Wrap(err)
			}

		}
	}

	id, err := identity.LoadIdentity(ctx, b.cfg.Outputs[0].GetDestination(), identity.BotKinds()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c, err := client.New(ctx, client.Config{
		Addrs:       []string{b.cfg.AuthServer},
		Credentials: []client.Credentials{clientCredentials{id}},
	})
	return c, trace.Wrap(err)
}

type clientCredentials struct {
	id *identity.Identity
}

func (c clientCredentials) Dialer(client.Config) (client.ContextDialer, error) {
	return nil, trace.NotImplemented("no dialer")
}

func (c clientCredentials) TLSConfig() (*tls.Config, error) {
	return c.id.TLSConfig(utils.DefaultCipherSuites())
}

func (c clientCredentials) SSHClientConfig() (*ssh.ClientConfig, error) {
	return c.id.SSHClientConfig(false)
}

func (b *Bot) NeedLeaderElection() bool {
	return true
}

func (b *Bot) Start(ctx context.Context) error {
	if b.getOnboardingConfig == nil {
		return trace.BadParameter("No bot onboarding config getter passed, cannot start the bot.")
	}
	onboarding, err := b.getOnboardingConfig(ctx, b.opts, b.rootClient)
	if err != nil {
		return trace.Wrap(err)
	}

	b.cfg.Onboarding = *onboarding

	realBot := tbot.New(b.cfg, log.StandardLogger())

	b.running = true
	log.Info("Running tbot")
	return trace.Wrap(realBot.Run(ctx))
}

// CreateAndBootstrapBot connects to teleport using a local auth connection, creates operator's role in teleport
// and creates tbot's configuration.
func CreateAndBootstrapBot(ctx context.Context, opts Options) (*Bot, *proto.Features, error) {
	if err := opts.CheckAndSetDefaults(); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// First we are creating a local auth client, like local tctl
	authClientConfig, err := createAuthClientConfig(opts)
	if err != nil {
		return nil, nil, trace.WrapWithMessage(err, "failed to create auth client config")
	}

	authClient, err := authclient.Connect(ctx, authClientConfig)
	if err != nil {
		return nil, nil, trace.WrapWithMessage(err, "failed to create auth client")
	}

	ping, err := authClient.Ping(ctx)
	if err != nil {
		return nil, nil, trace.WrapWithMessage(err, "failed to ping teleport")
	}

	// Then we create a role for the operator
	role, err := sidecarRole(opts.Role)
	if err != nil {
		return nil, nil, trace.WrapWithMessage(err, "failed to create role")
	}

	if err := authClient.UpsertRole(ctx, role); err != nil {
		return nil, nil, trace.WrapWithMessage(err, "failed to create operator's role")
	}
	log.Debug("Operator role created")

	bot := &Bot{
		running:             false,
		rootClient:          authClient,
		opts:                opts,
		getOnboardingConfig: sidecarKubeOnboardingConfig,
	}

	bot.initializeConfig(ctx)
	return bot, ping.ServerFeatures, nil
}

// sidecarTokenOnboardingConfig uses the sidecar local auth client to create an
// onboarding config doing a "token" join. As it is not currently possible to
// join back the cluster as an existing bot. (See https://github.com/gravitational/teleport/issues/13091)
// we must delete the previous bot and create a new one.
// This operation can cause race-conditions with the auth, eventually
// ending to the bot being locked and the operator broken.
func sidecarTokenOnboardingConfig(ctx context.Context, opts Options, authClient auth.ClientI) (*config.OnboardingConfig, error) {
	onboardingConfig := config.OnboardingConfig{
		JoinMethod: types.JoinMethodToken,
	}
	// We need to check if the bot exists first and cannot just attempt to delete
	// it because DeleteBot() returns an aggregate, which breaks the
	// ToGRPC/FromGRPC status code translation. We end up with the wrong error
	// type and cannot check if `trace.IsNotFound()`
	botRoleName := fmt.Sprintf("bot-%s", opts.Name)
	exists, err := botExists(ctx, opts, authClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if exists {
		err := authClient.DeleteBot(ctx, opts.Name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if err := authClient.DeleteRole(ctx, botRoleName); err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	response, err := authClient.CreateBot(ctx, &proto.CreateBotRequest{
		Name:  opts.Name,
		Roles: []string{opts.Role},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	onboardingConfig.TokenValue = response.TokenID

	caPins, err := getCAPins(ctx, authClient)
	onboardingConfig.CAPins = caPins

	return &onboardingConfig, nil
}

// sidecarKubeOnboardingConfig creates the bot's onboarding config to perform a Kubernetes join.
// It fetches the operator kube SA, validates it (this requires TokenReview permissions,
// but we currently have them as we're deployed with the auth SA), creates a Teleport
// provision token that allows the kubernetes SA, and finally create the bot that will
// use the token.
// Unlike with the preshared token join method, we don't need the delete/re-create the bot
// every time. If the bot is already here, we reuse it. The only exception is if the
// bot was previously used for renewable cert, we must destroy it to reset the generation
// label.
func sidecarKubeOnboardingConfig(ctx context.Context, opts Options, authClient auth.ClientI) (*config.OnboardingConfig, error) {
	caPins, err := getCAPins(ctx, authClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// We get the kubernetes SA token mounted in the container
	kubeSAToken, err := kubernetestoken.GetIDToken(os.Getenv, os.ReadFile)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "cannot read Kubernetes SA token")
	}

	// We validate that this token can be used and retrieve our SA name from it
	validator := kubernetestoken.TokenReviewValidator{}
	kubeSATokenInfo, err := validator.Validate(ctx, kubeSAToken)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "cannot validate the kubernetes SA token")
	}
	serviceAccount := strings.TrimPrefix(kubeSATokenInfo.Username, kubernetestoken.ServiceAccountNamePrefix+":")

	// We create/update the Teleport token to allow the operator bot to join
	// with the current Kubernetes SA token
	tokenSpec := types.ProvisionTokenSpecV2{
		Roles:      types.SystemRoles{types.RoleBot},
		JoinMethod: types.JoinMethodKubernetes,
		BotName:    opts.Name,
		Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
			Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
				{
					ServiceAccount: serviceAccount,
				},
			},
		},
	}
	teleportToken, err := types.NewProvisionTokenFromSpec(opts.Name, time.Time{}, tokenSpec)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = authClient.UpsertToken(ctx, teleportToken)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We check if the bot already exists and references the correct token name
	botUser, err := getTeleportBotUser(ctx, opts, authClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	onboardingConfig := &config.OnboardingConfig{
		TokenValue: opts.Name,
		CAPins:     caPins,
		JoinMethod: types.JoinMethodKubernetes,
	}

	if botUser != nil && botUser.BotGenerationLabel() != "0" {
		log.Infof("Found and old bot user %s with non-zero generation label, attempting to delete it", botUser)
		err = authClient.DeleteBot(ctx, opts.Name)
		if err != nil {
			return nil, trace.WrapWithMessage(err, "Error while deleting the old bot")
		}
		// the bot is no more, we need a new one
		botUser = nil
	}

	if botUser == nil {
		log.Infof("Found no bot user %s, creating a new one", botUser)
		_, err = authClient.CreateBot(ctx, &proto.CreateBotRequest{
			Name:    opts.Name,
			Roles:   []string{opts.Role},
			TokenID: opts.Name,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return onboardingConfig, nil
}

// getTeleportBotUser iterates over all bot users to find the one described in the Options.
// Returns nil if no bot is found.
func getTeleportBotUser(ctx context.Context, opts Options, authClient auth.ClientI) (types.User, error) {
	botUsers, err := authClient.GetBotUsers(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, botUser := range botUsers {
		if botUser.GetName() == fmt.Sprintf("bot-%s", opts.Name) {
			return botUser, nil
		}
	}
	return nil, nil

}

// botExists checks if the bot described in the Options exists
func botExists(ctx context.Context, opts Options, authClient auth.ClientI) (bool, error) {
	botUser, err := getTeleportBotUser(ctx, opts, authClient)
	return botUser != nil, trace.Wrap(err)
}

// getCAPins uses the local auth client to get the cluster CAs and compute their pins
func getCAPins(ctx context.Context, authClient auth.ClientI) ([]string, error) {
	// Getting the cluster CA Pins to be able to join regardless of the cert SANs.
	localCAResponse, err := authClient.GetClusterCACert(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	caPins, err := tlsca.CalculatePins(localCAResponse.TLSCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("CA Pins recovered: %s", caPins)
	return caPins, nil
}
