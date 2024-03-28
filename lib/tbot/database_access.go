/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
)

func getDatabase(ctx context.Context, clt *auth.Client, name string) (types.Database, error) {
	ctx, span := tracer.Start(ctx, "getDatabase")
	defer span.End()

	servers, err := apiclient.GetAllResources[types.DatabaseServer](ctx, clt, &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: makeNameOrDiscoveredNamePredicate(name),
		Limit:               int32(defaults.DefaultChunkSize),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases []types.Database
	for _, server := range servers {
		databases = append(databases, server.GetDatabase())
	}

	databases = types.DeduplicateDatabases(databases)
	db, err := chooseOneDatabase(databases, name)
	return db, trace.Wrap(err)
}

func getRouteToDatabase(
	ctx context.Context,
	log logrus.FieldLogger,
	client *auth.Client,
	service string,
	username string,
	database string,
) (proto.RouteToDatabase, error) {
	ctx, span := tracer.Start(ctx, "getRouteToDatabase")
	defer span.End()

	if service == "" {
		return proto.RouteToDatabase{}, nil
	}

	db, err := getDatabase(ctx, client, service)
	if err != nil {
		return proto.RouteToDatabase{}, trace.Wrap(err)
	}
	// make sure the output matches the fully resolved db name, since it may
	// have been just a "discovered name".
	service = db.GetName()
	if db.GetProtocol() == libdefaults.ProtocolMongoDB && username == "" {
		// This isn't strictly a runtime error so killing the process seems
		// wrong. We'll just loudly warn about it.
		log.Errorf("Database `username` field for %q is unset but is required for MongoDB databases.", service)
	} else if db.GetProtocol() == libdefaults.ProtocolRedis && username == "" {
		// Per tsh's lead, fall back to the default username.
		username = libdefaults.DefaultRedisUsername
	}

	return proto.RouteToDatabase{
		ServiceName: service,
		Protocol:    db.GetProtocol(),
		Database:    database,
		Username:    username,
	}, nil
}
