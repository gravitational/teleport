/*
Copyright 2019 Gravitational, Inc.

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

import { Gateway } from 'teleterm/ui/services/clusters';
import { routing } from 'teleterm/ui/uri';

export function useMySql(gateway: Gateway) {
  const clusterName = routing.parseClusterName(gateway.targetUri);
  const dbId = routing.parseDbUri(gateway.targetUri)?.params?.dbId;

  const mySqlConnArgs = [
    `mysql`,
    `--defaults-group-suffix=_${clusterName}-${dbId}`,
    `--host ${gateway.localAddress}`,
    `--port ${gateway.localPort}`,
    // MySQL CLI treats localhost as a special value and tries to use Unix Domain Socket for connection
    // To enforce TCP connection protocol needs to be explicitly specified.
    `--protocol TCP`,
    `--user [USER]`,
    `--database [DATABASE]`,
  ];

  if (gateway.insecure) {
    mySqlConnArgs.push(`--ssl-mode=VERIFY_CA`);
  }

  return mySqlConnArgs.join(' ');
}
