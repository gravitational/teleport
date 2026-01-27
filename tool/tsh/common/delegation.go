/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package common

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils/slices"
)

type delegationCommand struct {
	*kingpin.CmdClause

	profileName string
	sessionTTL  time.Duration
	resourceIDs []string
	botNames    []string
	format      string
}

func newDelegationCommand(app *kingpin.Application) *delegationCommand {
	cmd := &delegationCommand{
		CmdClause: app.Command("delegate-access", "Temporarily lend your access to a machine or workload."),
	}
	cmd.Flag("profile", "Name of the delegation profile that will be used.").Required().StringVar(&cmd.profileName)
	cmd.Flag("session-ttl", "How long access will be delegated to the machine or workload.").DurationVar(&cmd.sessionTTL)

	formats := []string{teleport.Text, teleport.JSON}
	cmd.Flag("format", defaults.FormatFlagDescription(formats...)).Short('f').Default(teleport.Text).EnumVar(&cmd.format, formats...)

	return cmd
}

func (c *delegationCommand) run(conf *CLIConf) error {
	tc, err := makeClient(conf)
	if err != nil {
		return trace.Wrap(err)
	}

	var profile *delegationv1.DelegationProfile
	err = client.RetryWithRelogin(conf.Context, tc, func() error {
		cl, err := tc.ConnectToCluster(conf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer cl.Close()

		profile, err = cl.AuthClient.DelegationProfileServiceClient().
			GetDelegationProfile(conf.Context, &delegationv1.GetDelegationProfileRequest{
				Name: c.profileName,
			})
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	confirmed, err := c.promptForConsent(conf, profile)
	if err != nil {
		return trace.Wrap(err)
	}
	if !confirmed {
		return nil
	}

	var session *delegationv1.DelegationSession
	err = client.RetryWithRelogin(conf.Context, tc, func() error {
		cl, err := tc.ConnectToCluster(conf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer cl.Close()

		req := &delegationv1.CreateDelegationSessionRequest{
			From: &delegationv1.CreateDelegationSessionRequest_Profile{
				Profile: &delegationv1.DelegationProfileReference{
					Name:     profile.GetMetadata().GetName(),
					Revision: profile.GetMetadata().GetRevision(),
				},
			},
		}
		if c.sessionTTL != 0 {
			req.Ttl = durationpb.New(c.sessionTTL)
		}

		session, err = cl.AuthClient.DelegationSessionServiceClient().
			CreateDelegationSession(conf.Context, req)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := c.printSession(conf, session); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (c *delegationCommand) promptForConsent(
	conf *CLIConf,
	profile *delegationv1.DelegationProfile,
) (bool, error) {
	var b strings.Builder
	fmt.Fprintln(&b, "Access delegation allows you to temporarily lend your identity to a machine or workload.")
	fmt.Fprintln(&b, "")

	fmt.Fprintln(&b, "The following bots:")
	for _, user := range profile.GetSpec().GetAuthorizedUsers() {
		fmt.Fprintf(&b, "- %s\n", user.GetBotName())
	}
	fmt.Fprintln(&b, "")

	fmt.Fprintln(&b, "Will be able to access the following resources on your behalf:")

	yml, err := yaml.Marshal(
		slices.Map(
			profile.GetSpec().GetRequiredResources(),
			func(res *delegationv1.DelegationResourceSpec) *delegationResourceMarshaller {
				return &delegationResourceMarshaller{res}
			},
		),
	)
	if err != nil {
		return false, trace.Wrap(err)
	}
	b.Write(yml)
	fmt.Fprintln(&b, "")

	fmt.Fprint(&b, "Are you sure you wish to proceed?")
	return prompt.Confirmation(
		conf.Context,
		conf.Stderr(),
		prompt.Stdin(),
		b.String(),
	)
}

func (c *delegationCommand) printSession(conf *CLIConf, session *delegationv1.DelegationSession) error {
	switch c.format {
	case teleport.JSON:
		if err := json.NewEncoder(conf.Stdout()).Encode(struct {
			SessionID string `json:"session_id"`
		}{session.GetMetadata().GetName()}); err != nil {
			return trace.Wrap(err)
		}
	default:
		fmt.Fprintf(
			conf.Stdout(),
			"Delegation session created. It will expire at: %s.\nProvide this Session ID to your workload or in your tbot configuration: %q\n",
			session.GetMetadata().GetExpires().AsTime().Format(time.RFC3339),
			session.GetMetadata().GetName(),
		)
	}

	return nil
}

type delegationResourceMarshaller struct {
	res *delegationv1.DelegationResourceSpec
}

func (m *delegationResourceMarshaller) MarshalJSON() ([]byte, error) {
	return protojson.MarshalOptions{UseProtoNames: true}.Marshal(m.res)
}
