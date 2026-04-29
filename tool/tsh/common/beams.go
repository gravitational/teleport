/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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
	"context"
	"fmt"
	"net"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/utils"
)

const beamsLogin = "beams"

type beamsCommands struct {
	ls        *beamsLSCommand
	add       *beamsAddCommand
	rm        *beamsRMCommand
	ssh       *beamsSSHCommand
	exec      *beamsExecCommand
	publish   *beamsPublishCommand
	unpublish *beamsUnpublishCommand
	scp       *beamsSCPCommand
}

func newBeamsCommands(app *kingpin.Application) beamsCommands {
	beams := app.Command("beams", "View, manage and run beams. Beams are ephemeral, sandbox VMs built for agentic workloads.").Alias("beam")
	return beamsCommands{
		ls:        newBeamsLSCommand(beams),
		add:       newBeamsAddCommand(beams),
		rm:        newBeamsRMCommand(beams),
		ssh:       newBeamsSSHCommand(beams),
		exec:      newBeamsExecCommand(beams),
		publish:   newBeamsPublishCommand(beams),
		unpublish: newBeamsUnpublishCommand(beams),
		scp:       newBeamsSCPCommand(beams),
	}
}

func formatBeam(beam *beamsv1.Beam, proxyAddr string) formattedBeam {
	return formattedBeam{
		ID:      beam.GetStatus().GetAlias(),
		UUID:    beam.GetMetadata().GetName(),
		Owner:   beam.GetStatus().GetUser(),
		Expires: beam.GetSpec().GetExpires().AsTime(),
		URL:     beamPublishURL(beam, proxyAddr),
	}
}

func beamPublishURL(beam *beamsv1.Beam, proxyAddr string) string {
	publish := beam.GetSpec().GetPublish()
	if publish == nil {
		return ""
	}

	// We discard the port because in Teleport Cloud proxies are always listening
	// on :443. For TCP apps, the address can only be dialed via VNet anyway (and
	// the port doesn't need to match the proxy port).
	proxyHost, _, err := net.SplitHostPort(proxyAddr)
	if err != nil {
		proxyHost = proxyAddr
	}

	hostname := utils.DefaultAppPublicAddr(beam.GetStatus().GetAppName(), proxyHost)
	switch publish.GetProtocol() {
	case beamsv1.Protocol_PROTOCOL_HTTP:
		return fmt.Sprintf("https://%s", hostname)
	case beamsv1.Protocol_PROTOCOL_TCP:
		return fmt.Sprintf("tcp://%s:%d", hostname, publish.GetPort())
	default:
		return hostname
	}
}

// formattedBeam contains only the useful parts of the beam resource for scripting
// or automating with an AI Agent.
type formattedBeam struct {
	// ID is the human-friendly name of the beam (taken from `status.alias`).
	//
	// As there is a limited number of possible word pairs, eventual reuse is
	// likely but it will uniquely point to this beam for as long as the beam
	// is alive.
	ID string `json:"id"`

	// UUID is the stable/globally-unique identifier for the beam (taken from
	// `metadata.name`).
	UUID string `json:"uuid"`

	// Owner of this beam (taken from `status.user`).
	Owner string `json:"owner"`

	// Expires is the time at which this beam will expire and be automatically
	// deleted (taken from `spec.expires`).
	Expires time.Time `json:"expires"`

	// URL is the address at which the beam's published application can be
	// reached.
	//
	// For HTTP applications, it will be in the form: `https://<host>`.
	//
	// For TCP applications, it will be in the form: `tcp://<host>:<port>` but
	// this address can only be dialed via VNet, otherwise you'll need to start
	// a local proxy.
	URL string `json:"url,omitempty"`
}

// getBeam reads a beam by UUID or human-friendly name depending on the format
// of the given string.
func getBeam(ctx context.Context, client authclient.ClientI, ref string) (*beamsv1.Beam, error) {
	req := &beamsv1.GetBeamRequest{}
	if _, err := uuid.Parse(ref); err == nil {
		req.Id = &beamsv1.GetBeamRequest_Name{Name: ref}
	} else {
		req.Id = &beamsv1.GetBeamRequest_Alias{Alias: ref}
	}

	rsp, err := client.
		BeamServiceClient().
		GetBeam(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp.GetBeam(), nil
}
