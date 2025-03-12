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
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/lib/client"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type databaseExecCommand struct {
	cf            *CLIConf
	tc            *client.TeleportClient
	clusterClient *client.ClusterClient
	profile       *client.ProfileStatus
	tracer        oteltrace.Tracer
	closers       []func()
}

func (c *databaseExecCommand) run(cf *CLIConf) error {
	defer c.close()

	if err := c.init(cf); err != nil {
		return trace.Wrap(err)
	}

	dbs, err := c.getDatabases()
	if err != nil {
		return trace.Wrap(err)
	}

	logger.DebugContext(cf.Context, "Fetched database services.", "databases", logutils.IterAttr(types.ResourceNameIter(dbs)))

	group, groupCtx := errgroup.WithContext(c.cf.Context)
	group.SetLimit(c.cf.MaxConnections)
	for _, db := range dbs {
		// TODO(greedy52)
		fmt.Println(groupCtx)
		fmt.Println(db)
	}
	return nil
}

func (c *databaseExecCommand) init(cf *CLIConf) (err error) {
	if err := c.checkFlags(cf); err != nil {
		return trace.Wrap(err)
	}

	c.cf = cf
	c.tracer = c.cf.TracingProvider.Tracer(teleport.ComponentTSH)
	c.tc, err = makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := client.RetryWithRelogin(cf.Context, c.tc, func() error {
		c.clusterClient, err = c.tc.ConnectToCluster(cf.Context)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

	c.addCloser(func() {
		c.clusterClient.Close()
	})

	c.profile, err = c.tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (c *databaseExecCommand) addCloser(closer func()) {
	c.closers = append(c.closers, closer)
}

func (c *databaseExecCommand) close() {
	for _, closer := range c.closers {
		closer()
	}
}

func (c *databaseExecCommand) checkFlags(cf *CLIConf) error {
	if cf.MaxConnections <= 0 && cf.MaxConnections > 10 {
		return trace.BadParameter("--max-connections must be between 1 and 10")
	}

	// selection flags
	byNames := cf.DatabaseServices != ""
	bySearch := cf.SearchKeywords != "" || cf.Labels != ""
	switch {
	case !byNames && !bySearch:
		return trace.BadParameter("please provide one of --dbs, --labels, --search flag")
	case byNames && bySearch:
		return trace.BadParameter("--labels/--search flags cannot be used with --dbs flag")
	}
	return nil
}

func (c *databaseExecCommand) getDatabases() ([]types.Database, error) {
	if c.cf.DatabaseServices != "" {
		return c.getDatabasesByNames()
	}
	return c.searchDatabases()
}

func (c *databaseExecCommand) getDatabasesByNames() ([]types.Database, error) {
	group, groupCtx := errgroup.WithContext(c.cf.Context)
	group.SetLimit(c.cf.MaxConnections)

	var (
		mu  sync.Mutex
		dbs []types.Database
	)
	for _, name := range strings.Split(c.cf.DatabaseServices, ",") {
		group.Go(func() error {
			list, err := c.listDatabasesWithFilter(groupCtx, &proto.ListResourcesRequest{
				Namespace:           apidefaults.Namespace,
				ResourceType:        types.KindDatabaseServer,
				PredicateExpression: makeNamePredicate(name),
			})
			if err != nil {
				return trace.Wrap(err)
			}
			switch len(list) {
			case 0:
				return trace.NotFound("database %q not found", name)
			case 1:
				mu.Lock()
				defer mu.Unlock()
				dbs = append(dbs, list[0])
				return nil
			default:
				return trace.CompareFailed("expecting one database but got %d", len(list))
			}
		})
	}

	if err := group.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := c.checkDatabases(dbs); err != nil {
		return nil, trace.Wrap(err)
	}
	return dbs, nil
}

func (c *databaseExecCommand) searchDatabases() (databases []types.Database, err error) {
	dbs, err := c.listDatabasesWithFilter(c.cf.Context, c.tc.ResourceFilter(types.KindDatabaseServer))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := c.checkDatabases(dbs); err != nil {
		return nil, trace.Wrap(err)
	}

	// Print results.
	fmt.Fprintf(c.cf.Stdout(), "Found %d databases:\n\n", len(dbs))
	var rows []databaseTableRow
	for _, db := range dbs {
		rows = append(rows, getDatabaseRow("", "", "", db, nil, nil, false))
	}
	printDatabaseTable(printDatabaseTableConfig{
		writer:         c.cf.Stdout(),
		rows:           rows,
		includeColumns: []string{"Name", "Protocol", "Description", "Labels"},
	})

	// Prompt.
	if !c.cf.SkipConfirm {
		ok, err := prompt.Confirmation(c.cf.Context, c.cf.Stdout(), prompt.NewContextReader(c.cf.Stdin()), "Do you want to continue?")
		if err != nil {
			return nil, trace.Wrap(err)
		} else if !ok {
			return nil, trace.Errorf("Exec canceled.")
		}
	}
	return dbs, nil
}

func (c *databaseExecCommand) listDatabasesWithFilter(ctx context.Context, filter *proto.ListResourcesRequest) (databases []types.Database, err error) {
	ctx, span := c.tracer.Start(
		ctx,
		"listDatabasesWithFilter",
		oteltrace.WithSpanKind(oteltrace.SpanKindClient),
	)
	defer span.End()

	servers, err := apiclient.GetAllResources[types.DatabaseServer](ctx, c.clusterClient.AuthClient, filter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return types.DatabaseServers(servers).ToDatabases(), nil
}

func (c *databaseExecCommand) checkDatabases(dbs []types.Database) error {
	for _, db := range dbs {
		if isDatabaseUserRequired(db.GetProtocol()) && c.cf.DatabaseUser == "" {
			return trace.BadParameter("--db-user is required for database %s", db.GetName())
		}
		if isDatabaseNameRequired(db.GetProtocol()) && c.cf.DatabaseName == "" {
			return trace.BadParameter("--db-name is required for database %s", db.GetName())
		}
	}
	return nil
}
