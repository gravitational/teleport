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

package mongodb

import (
	"context"
	"crypto/tls"
	"iter"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/gravitational/trace"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/mongo/address"
	"go.mongodb.org/mongo-driver/mongo/description"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/x/mongo/driver"
	"go.mongodb.org/mongo-driver/x/mongo/driver/auth"
	"go.mongodb.org/mongo-driver/x/mongo/driver/connstring"
	"go.mongodb.org/mongo-driver/x/mongo/driver/ocsp"
	"go.mongodb.org/mongo-driver/x/mongo/driver/topology"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/endpoints"
	"github.com/gravitational/teleport/lib/srv/db/mongodb/protocol"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

const (
	// awsSecretTokenKey is the authenticator property name used to pass AWS
	// session token. This name is defined by the mongo driver.
	awsSecretTokenKey = "AWS_SESSION_TOKEN"
	// awsIAMSource is the authenticator source value used when authenticating
	// using AWS IAM.
	// https://www.mongodb.com/docs/manual/reference/connection-string/#mongodb-urioption-urioption.authSource
	awsIAMSource = "$external"
)

// connect returns connection to a MongoDB server.
//
// When connecting to a replica set, returns connection to the server selected
// based on the read preference connection string option. This allows users to
// configure database access to always connect to a secondary for example.
func (e *Engine) connect(ctx context.Context, sessionCtx *common.Session) (*serverConnection, error) {
	deployment, err := depCache.load(ctx, sessionCtx, e.connectDeployment)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn, err := deployment.connect(ctx)
	if err != nil {
		return nil, trace.NewAggregate(err, deployment.close(ctx))
	}
	return conn, nil
}

func (e *Engine) connectDeployment(ctx context.Context, sessionCtx *common.Session) (*deployment, error) {
	options, selector, err := e.getTopologyOptions(ctx, sessionCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Using driver's "topology" package allows to retain low-level control
	// over server connections (reading/writing wire messages) but at the
	// same time get access to logic such as picking a server to connect to
	// in a replica set.
	top, err := topology.New(options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := top.Connect(); err != nil {
		e.Log.DebugContext(e.Context, "Failed to connect topology", "error", err)
		return nil, trace.Wrap(err)
	}
	return &deployment{
		top:              top,
		selector:         selector,
		log:              e.Log,
		messagesReceived: common.GetMessagesFromServerMetric(sessionCtx.Database),
	}, nil
}

// getTopologyOptions constructs topology options for connecting to a MongoDB server.
func (e *Engine) getTopologyOptions(ctx context.Context, sessionCtx *common.Session) (*topology.Config, description.ServerSelector, error) {
	clientCfg, err := makeClientOptionsFromDatabaseURI(sessionCtx.Database.GetURI())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	topoConfig, err := topology.NewConfig(clientCfg, nil)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	serverOptions, err := e.getServerOptions(ctx, sessionCtx, clientCfg)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	topoConfig.ServerOpts = serverOptions

	selector, err := getServerSelector(clientCfg)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return topoConfig, selector, nil
}

// getServerOptions constructs server options for connecting to a MongoDB server.
func (e *Engine) getServerOptions(ctx context.Context, sessionCtx *common.Session, clientCfg *options.ClientOptions) ([]topology.ServerOption, error) {
	connectionOptions, err := e.getConnectionOptions(ctx, sessionCtx, clientCfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []topology.ServerOption{
		topology.WithConnectionOptions(func(opts ...topology.ConnectionOption) []topology.ConnectionOption {
			return connectionOptions
		}),
	}, nil
}

// getConnectionOptions constructs connection options for connecting to a MongoDB server.
func (e *Engine) getConnectionOptions(ctx context.Context, sessionCtx *common.Session, clientCfg *options.ClientOptions) ([]topology.ConnectionOption, error) {
	tlsConfig, err := e.Auth.GetTLSConfig(ctx, sessionCtx.GetExpiry(), sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authenticator, err := e.getAuthenticator(ctx, sessionCtx, clientCfg.HTTPClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []topology.ConnectionOption{
		topology.WithTLSConfig(func(*tls.Config) *tls.Config {
			return tlsConfig
		}),
		topology.WithOCSPCache(func(ocsp.Cache) ocsp.Cache {
			return ocsp.NewCache()
		}),
		topology.WithHandshaker(func(topology.Handshaker) topology.Handshaker {
			// Auth handshaker will authenticate the client connection using
			// x509 mechanism as the database user specified above.
			return auth.Handshaker(
				// Wrap the driver's auth handshaker with our custom no-op
				// handshaker to prevent the driver from sending client metadata
				// to the server as a first message. Otherwise, the actual
				// client connecting to Teleport will get an error when they try
				// to send its own metadata since client metadata is immutable.
				&handshaker{},
				&auth.HandshakeOptions{Authenticator: authenticator})
		}),
	}, nil
}

func (e *Engine) getAuthenticator(ctx context.Context, sessionCtx *common.Session, httpClient *http.Client) (auth.Authenticator, error) {
	dbType := sessionCtx.Database.GetType()
	isAtlasDB := dbType == types.DatabaseTypeMongoAtlas

	// Currently, the MongoDB Atlas IAM Authentication doesn't work with IAM
	// users. Here we provide a better error message to the users.
	if isAtlasDB && awsutils.IsUserARN(sessionCtx.DatabaseUser) {
		return nil, trace.BadParameter("MongoDB Atlas AWS IAM Authentication with IAM users is not supported.")
	}

	switch {
	case isAtlasDB && awsutils.IsRoleARN(sessionCtx.DatabaseUser):
		return e.getAWSAuthenticator(ctx, sessionCtx, httpClient)
	case dbType == types.DatabaseTypeDocumentDB:
		// DocumentDB uses the same the IAM authenticator as MongoDB Atlas.
		return e.getAWSAuthenticator(ctx, sessionCtx, httpClient)
	default:
		e.Log.DebugContext(e.Context, "Authenticating to database using certificates.")
		authenticator, err := auth.CreateAuthenticator(auth.MongoDBX509, &auth.Cred{
			Username: x509Username(sessionCtx),
		}, httpClient)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return authenticator, nil
	}
}

// getAWSAuthenticator fetches the AWS credentials and initializes the MongoDB
// authenticator.
func (e *Engine) getAWSAuthenticator(ctx context.Context, sessionCtx *common.Session, httpClient *http.Client) (auth.Authenticator, error) {
	e.Log.DebugContext(e.Context, "Authenticating to database using AWS IAM authentication.")

	username, password, sessToken, err := e.Auth.GetAWSIAMCreds(ctx, sessionCtx.Database, sessionCtx.DatabaseUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authenticator, err := auth.CreateAuthenticator(auth.MongoDBAWS, &auth.Cred{
		Source:   awsIAMSource,
		Username: username,
		Password: password,
		Props: map[string]string{
			awsSecretTokenKey: sessToken,
		},
	}, httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return authenticator, nil
}

func makeClientOptionsFromDatabaseURI(uri string) (*options.ClientOptions, error) {
	clientCfg := options.Client()
	clientCfg.SetServerSelectionTimeout(common.DefaultMongoDBServerSelectionTimeout)
	if strings.HasPrefix(uri, connstring.SchemeMongoDB) ||
		strings.HasPrefix(uri, connstring.SchemeMongoDBSRV) {
		clientCfg.ApplyURI(uri)
	} else {
		clientCfg.Hosts = []string{uri}
	}
	if err := clientCfg.Validate(); err != nil {
		return nil, trace.Wrap(err)
	}
	return clientCfg, nil
}

// NewEndpointsResolver returns a health check target endpoint resolver.
// SRV URI (mongodb+srv://) is resolved to a seed list from DNS SRV record
// https://www.mongodb.com/docs/manual/reference/connection-string/#srv-connection-format
func NewEndpointsResolver(_ context.Context, db types.Database, _ endpoints.ResolverBuilderConfig) (endpoints.Resolver, error) {
	return newEndpointsResolver(db.GetURI()), nil
}

func newEndpointsResolver(uri string) endpoints.Resolver {
	return endpoints.ResolverFn(func(ctx context.Context) ([]string, error) {
		clientCfg, err := makeClientOptionsFromDatabaseURI(uri)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		endpoints := make([]string, 0, len(clientCfg.Hosts))
		for _, host := range clientCfg.Hosts {
			endpoints = append(endpoints, address.Address(host).String())
		}
		return endpoints, nil
	})
}

// getServerSelector returns selector for picking the server to connect to,
// which is mostly useful when connecting to a MongoDB replica set.
//
// It uses readPreference connection flag. Defaults to "primary".
func getServerSelector(clientOptions *options.ClientOptions) (description.ServerSelector, error) {
	if clientOptions.ReadPreference == nil {
		return description.ReadPrefSelector(readpref.Primary()), nil
	}
	readPref, err := readpref.New(clientOptions.ReadPreference.Mode())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return description.ReadPrefSelector(readPref), nil
}

// handshaker is Mongo driver no-op handshaker that doesn't send client
// metadata when connecting to server.
type handshaker struct{}

// GetHandshakeInformation overrides default auth handshaker's logic which
// would otherwise have sent client metadata request to the server which
// would break the actual client connecting to Teleport.
func (h *handshaker) GetHandshakeInformation(context.Context, address.Address, driver.Connection) (driver.HandshakeInformation, error) {
	return driver.HandshakeInformation{}, nil
}

// Finish handshake is no-op as all auth logic will be done by the driver's
// default auth handshaker.
func (h *handshaker) FinishHandshake(context.Context, driver.Connection) error {
	return nil
}

func x509Username(sessionCtx *common.Session) string {
	// MongoDB uses full certificate Subject field as a username.
	return "CN=" + sessionCtx.DatabaseUser
}

type serverConnection struct {
	connection driver.Connection
	deployment *deployment
}

type replyIterator iter.Seq2[protocol.Message, error]

func (c *serverConnection) roundTrip(ctx context.Context, clientMessage protocol.Message, maxMessageSize uint32) replyIterator {
	return func(yield func(protocol.Message, error) bool) {
		if err := c.writeMessage(ctx, clientMessage); err != nil {
			yield(nil, err)
			return
		}
		// Some client messages will not receive a reply.
		if clientMessage.MoreToCome(nil) {
			return
		}
		for {
			// Otherwise read the server's reply...
			serverMessage, err := c.readMessage(ctx, maxMessageSize)
			if !yield(serverMessage, err) || err != nil {
				return
			}
			// Stop reading if server indicated it has nothing more to send.
			if !serverMessage.MoreToCome(clientMessage) {
				return
			}
		}
	}
}

func (c *serverConnection) readMessage(ctx context.Context, maxMessageSize uint32) (protocol.Message, error) {
	msg, err := protocol.ReadServerMessage(ctx, c.connection, maxMessageSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.deployment.messagesReceived.Inc()
	return msg, nil
}

func (c *serverConnection) writeMessage(ctx context.Context, clientMessage protocol.Message) error {
	return trace.Wrap(c.connection.WriteWireMessage(ctx, clientMessage.GetBytes()))
}

func (c *serverConnection) close(ctx context.Context) error {
	var errs []error
	if err := c.deployment.close(ctx); err != nil {
		errs = append(errs, err)
	}
	if expirer, ok := c.connection.(driver.Expirable); ok {
		// expire the connection to evict it from the driver connection pool
		if err := expirer.Expire(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.connection.Close(); err != nil {
		errs = append(errs, err)
	}
	return trace.NewAggregate(errs...)
}

type clientConnection struct {
	net.Conn
	messagesReceived prometheus.Counter
}

func (c *clientConnection) readMessage(maxMessageSize uint32) (protocol.Message, error) {
	msg, err := protocol.ReadMessage(c, maxMessageSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	c.messagesReceived.Inc()
	return msg, trace.Wrap(err)
}

func (c *clientConnection) writeMessage(message protocol.Message) error {
	_, err := c.Write(message.GetBytes())
	return trace.Wrap(err)
}

// deployment wraps [topology.Topology] and [description.ServerSelector].
// It is used to select and connect to a server in the topology.
type deployment struct {
	top      *topology.Topology
	selector description.ServerSelector

	onClose          func(ctx context.Context, log *slog.Logger) error
	log              *slog.Logger
	messagesReceived prometheus.Counter
}

// connect selects a server, connects to it, and returns the connection.
func (d *deployment) connect(ctx context.Context) (*serverConnection, error) {
	d.log.DebugContext(ctx, "Selecting server from topology", "topology", d.top)
	server, err := d.top.SelectServer(ctx, d.selector)
	if err != nil {
		d.log.DebugContext(ctx, "Failed to select server from topology", "error", err)
		return nil, trace.Wrap(err)
	}

	d.log.DebugContext(ctx, "Connecting to server", "server", server)
	conn, err := server.Connection(ctx)
	if err != nil {
		d.log.DebugContext(ctx, "Failed to connect to server", "error", err)
		return nil, trace.Wrap(err)
	}
	serverConn := &serverConnection{
		connection: conn,
		deployment: d,
	}
	return serverConn, nil
}

func (d *deployment) close(ctx context.Context) error {
	if d.onClose != nil {
		return trace.Wrap(d.onClose(ctx, d.log))
	}
	return nil
}
