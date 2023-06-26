// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import "fmt"

const (
	usageNotes = `Notes:
  --roles=node,proxy,auth,app

  This flag tells Teleport which services to run. By default it runs auth,
  proxy, and node. In a production environment you may want to separate them.

  --token=xyz

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
  allows access to users with the role "group=dev".`

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
region us-west-1 which only allows access to users with the role "env=aws".`

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
)

const (
	sampleConfComment = `#
# A Sample Teleport configuration file.
#
# Things to update:
#  1. license.pem: You only need a license from https://dashboard.goteleport.com
#     if you are an Enterprise customer.
#`
)
