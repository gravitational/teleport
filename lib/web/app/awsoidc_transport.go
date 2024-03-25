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

package app

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	awssession "github.com/aws/aws-sdk-go/aws/session"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/recorder"
	"github.com/gravitational/teleport/lib/integrations/awsoidc"
	rsession "github.com/gravitational/teleport/lib/session"
	srvApp "github.com/gravitational/teleport/lib/srv/app"
	appaws "github.com/gravitational/teleport/lib/srv/app/aws"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/tlsca"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

func roundTripperForAWSOIDCIntegration(ctx context.Context, c *transportConfig) (http.RoundTripper, error) {
	app := c.servers[0].GetApp()
	awsoidcIntegrationName := app.GetIntegration()
	chunkId := uuid.NewString()

	remoteSiteClient, err := c.proxyClient.GetSite(c.identity.RouteToCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clt, err := remoteSiteClient.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessionV1, err := awsoidc.NewSessionV1(ctx, clt, "" /* no region */, awsoidcIntegrationName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsSigner, err := awsutils.NewSigningService(awsutils.SigningServiceConfig{
		Session: sessionV1,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	startTime := srvApp.SessionStartTime(ctx, c.log)
	// if startTime.IsZero() {
	// 	startTime = time.Now()
	// }

	// TODO(marco): Fix Audit
	// Create the stream writer that will write this chunk to the audit log.
	// Audit stream is using server context, not session context,
	// to make sure that session is uploaded even after it is closed.
	rec, err := newSessionRecorder(c.closingCtx, startTime, c.serverID, c.identity.RouteToApp.SessionID, c.datadir, clt)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	audit, err := common.NewAudit(common.AuditConfig{
		Emitter:  clt,
		Recorder: rec,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sessCtx := &common.SessionContext{
		Identity: c.identity,
		App:      app,
		ChunkID:  chunkId,
		Audit:    audit,
	}

	return &awsoidcRoundTripper{
		closingCtx: c.closingCtx,
		awsSigner:  awsSigner,
		sessCtx:    sessCtx,
		clock:      clockwork.NewRealClock(),
		serverID:   c.serverID,
		log:        c.log,
		rec:        rec,
	}, nil
}

// newSessionRecorder creates a session stream that will be used to record
// requests that occur within this session chunk and upload the recording
// to the Auth server.
func newSessionRecorder(ctx context.Context, startTime time.Time, serverID, chunkID, datadir string, clt auth.ClientI) (events.SessionPreparerRecorder, error) {
	recConfig, err := clt.GetSessionRecordingConfig(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterName, err := clt.GetClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	rec, err := recorder.New(recorder.Config{
		SessionID:    rsession.ID(chunkID),
		ServerID:     serverID,
		Namespace:    apidefaults.Namespace,
		Clock:        clockwork.NewRealClock(),
		ClusterName:  clusterName.GetClusterName(),
		RecordingCfg: recConfig,
		SyncStreamer: clt,
		DataDir:      datadir,
		Component:    teleport.Component(teleport.ComponentSession, teleport.ComponentProxy),
		Context:      ctx,
		StartTime:    startTime,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return rec, nil
}

type awsoidcRoundTripper struct {
	// closingCtx is the Server/Process context.Context.
	closingCtx context.Context

	sessCtx   *common.SessionContext
	awsSigner *awsutils.SigningService
	clock     clockwork.Clock
	serverID  string

	log logrus.FieldLogger

	rec events.SessionPreparerRecorder

	startChunkOnce sync.Once
}

func (a *awsoidcRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	ctx := r.Context()
	app := a.sessCtx.App
	identity := a.sessCtx.Identity

	// For ConsoleSignIn links, Teleport only creates and returns the sign in link.
	if r.Header.Get(common.TeleportAWSAssumedRole) == "" && !awsutils.IsSignedByAWSSigV4(r) {
		redirectResp, err := awsConsoleSignInResponse(a.awsSigner.Session, app, identity)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return redirectResp, nil
	}

	// Session Chunk events events are emitted when Teleport proxies the requests.
	// A single Chunk must be created per session and multiple Requests will be added to it.
	// TODO(marco): on error, the request must not continue
	a.startChunkOnce.Do(func() {
		if err := a.sessCtx.Audit.OnSessionChunk(ctx, a.serverID, a.sessCtx.ChunkID, a.sessCtx.Identity, app); err != nil {
			a.log.WithError(err).Warn("Failed to emit audit event.")
			return
		}

		go func() {
			// wait for server to close before explicitly closing the recorder
			<-a.closingCtx.Done()
			a.log.Warn("===== Closing Recorder because server is closing down")
			if err := a.rec.Close(a.closingCtx); err != nil {
				a.log.WithError(err).Warn("Failed to close recorder.")
			}
		}()
	})

	r = common.WithSessionContext(r, a.sessCtx)

	re, err := appaws.ResolveEndpoint(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	unsignedReq, responseExtraHeaders, err := appaws.RewriteCommonRequest(ctx, a.clock, a.sessCtx, r, re)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	signedReq, err := a.awsSigner.SignRequest(ctx, unsignedReq,
		&awsutils.SigningCtx{
			SigningName:   re.SigningName,
			SigningRegion: re.SigningRegion,
			Expiry:        identity.Expires,
			SessionName:   identity.Username,
			AWSRoleArn:    identity.RouteToApp.AWSRoleARN,
			AWSExternalID: app.GetAWSExternalID(),
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	httpClt, err := defaults.HTTPClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := httpClt.Do(signedReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for headerKey, headerValue := range responseExtraHeaders {
		resp.Header.Set(headerKey, strings.Join(headerValue, ";"))
	}

	log.Printf("TODO(marco): emitting Request Event status=%q resolvedEndpoint=%q", resp.Status, re.URL)
	if err := appaws.EmitAudit(a.closingCtx, a.sessCtx, unsignedReq, uint32(resp.StatusCode), re); err != nil {
		// log but don't return the error, because we already handed off request/response handling to the oxy forwarder.
		a.log.WithError(err).Warn("Failed to emit audit event.")
	}

	// if err := a.rec.Close(a.closingCtx); err != nil {
	// 	a.log.WithError(err).Warn("Failed to close recorder.")
	// }

	return resp, nil
}

func awsConsoleSignInResponse(awsSession *awssession.Session, app types.Application, identity *tlsca.Identity) (*http.Response, error) {
	srvAppCloud, err := srvApp.NewCloud(srvApp.CloudConfig{
		Session: awsSession,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signedLoginURL, err := srvAppCloud.GetAWSSigninURL(srvApp.AWSSigninRequest{
		Identity:   identity,
		TargetURL:  app.GetURI(),
		Issuer:     app.GetPublicAddr(),
		ExternalID: app.GetAWSExternalID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &http.Response{
		StatusCode: http.StatusFound,
		Status:     http.StatusText(http.StatusFound),
		Header: http.Header{
			"Location": []string{signedLoginURL.SigninURL},
		},
		Body: io.NopCloser(strings.NewReader(signedLoginURL.SigninURL)),
	}, nil
}
