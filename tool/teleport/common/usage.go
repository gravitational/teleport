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

package common

import "fmt"

const (
	usageNotes = `Notes:
  --roles=node,proxy,auth,app

  This flag tells Teleport which services to run. By default it runs auth,
  proxy, and node. In a production environment you may want to separate them.

  --token=xyz or --token=/tmp/token

  This token is needed to connect a node or web app to an auth server. Get it
  by running "tctl tokens add --type=node" or "tctl tokens add --type=app" to
  join an SSH server or web app to your cluster respectively. It's used once
  and ignored afterwards.
`

	appUsageExamples = `
> teleport app start --token=xyz --auth-server=proxy.example.com:3080 \
    --name="example-app" \
    --uri="http://localhost:8080"
  Starts an app server that proxies the application "example-app" running at
  http://localhost:8080.

> teleport app start --token=xyz --auth-server=proxy.example.com:3080 \
    --name="example-app" \
    --uri="http://localhost:8080" \
    --labels=group=dev
  Same as the above, but the app server runs with "group=dev" label which only
  allows access to users with the application label "group: dev" in an assigned role.`

	dbUsageExamples = `
> teleport db start --token=xyz --auth-server=proxy.example.com:3080 \
  --name="example-db" \
  --protocol="postgres" \
  --uri="localhost:5432"
  Starts a database server that proxies PostgreSQL database "example-db" running
  at localhost:5432. The database must be configured with Teleport CA and key
  pair issued by "tctl auth sign --format=db".

> teleport db start --token=xyz --auth-server=proxy.example.com:3080 \
  --name="aurora-db" \
  --protocol="mysql" \
  --uri="example.cluster-abcdefghij.us-west-1.rds.amazonaws.com:3306" \
  --aws-region=us-west-1 \
  --labels=env=aws
  Starts a database server that proxies Aurora MySQL database running in AWS
  region us-west-1 which only allows access to users with the database label
  "env: aws" in an assigned role.`

	systemdInstallExamples = `
  > teleport install systemd
  Generates a systemd unit file with the default configuration and outputs it to the terminal.

  > teleport install systemd \
    --fd-limit=8192 \
    --env-file=/etc/default/teleport \
    --pid-file=/run/teleport.pid \
    --teleport-path=/usr/local/bin/teleport \
    --output=/etc/systemd/system/teleport.service
  Generates a systemd unit file teleport.service using the provided flags and 
  places it in the given system configuration directory.
`

	dbCreateConfigExamples = `
> teleport db configure create --rds-discovery=us-west-1 --rds-discovery=us-west-2
Generates a configuration with samples and Aurora/RDS auto-discovery enabled on
the "us-west-1" and "us-west-2" regions.

> teleport db configure create \
   --token=/tmp/token \
   --proxy=localhost:3080 \
   --name=sample-db \
   --protocol=postgres \
   --uri=localhost:5432 \
   --labels=env=prod
Generates a configuration with a Postgres database.

> teleport db configure create --output file:///etc/teleport.yaml
Generates a configuration with samples and write to "/etc/teleport.yaml".`
)

var (
	usageExamples = fmt.Sprintf(`
Examples:

> teleport start
  By default without any configuration, teleport starts running as a single-node
  cluster. It's the equivalent of running with --roles=node,proxy,auth

> teleport start --roles=node --auth-server=10.1.0.1 --token=xyz --nodename=db
  Starts a node named 'db' running in strictly SSH mode role, joining the cluster
  serviced by the auth server running on 10.1.0.1

> teleport start --roles=node --auth-server=10.1.0.1 --labels=db=master
  Same as the above, but the node runs with db=master label and can be connected
  to using that label in addition to its name.
%v
%v`, appUsageExamples, dbUsageExamples)

	collectProfileUsageExamples = `Examples:
> teleport debug profile > pprof.tar.gz
  Collect default profiles and stores the resulting tarball in a file.

> teleport debug profile | tar xzv -C pprof/
  Collects default profiles, decompress the results into pprof/ directory.

> teleport debug profile heap,goroutine > pprof.tar.gz
  Collects heap and goroutine profiles. Stores the results in a file.

> teleport debug profile -s 0 goroutine > pprof.tar.gz
  Collects a snapshot of goroutine profile. Stores the results in a file.
  Note: Only allocs,block,goroutine,heap,mutex,threadcreate,trace,cmdline
        supports snapshots.`
)

const (
	sampleConfComment = `#
# A Sample Teleport configuration file.
#
# Things to update:
#  1. license.pem: Retrieve a license from your Teleport account https://teleport.sh
#     if you are an Enterprise customer.
#`
)
