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

export function useMongo(gateway: Gateway) {
  const mongoConnectionStr = [
    `mongo`,
    `--host ${gateway.localAddress}`,
    `--port ${gateway.localPort}`,
    `--ssl`,
    `--sslPEMKeyFile ${gateway.certPath}`,
    `--sslCAFile ${gateway.caCertPath}`,
  ].join(' ');

  return mongoConnectionStr;
}
