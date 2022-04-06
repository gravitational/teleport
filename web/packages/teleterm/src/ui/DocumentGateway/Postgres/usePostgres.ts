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

// SSLModeVerifyFull is the Postgres SSL "verify-full" mode.
//
// See Postgres SSL docs for more info:
// https://www.postgresql.org/docs/current/libpq-ssl.html
const SSL_MODE_VERIFY_FULL = 'verify-full';
// SSLModeVerifyCA is the Postgres SSL "verify-ca" mode.
//
// See Postgres SSL docs for more info:
// https://www.postgresql.org/docs/current/libpq-ssl.html
const SSL_MODE_VERIFY_CA = 'verify-ca';

export function usePostgres(gateway: Gateway) {
  const args = [
    `sslrootcert=${gateway.caCertPath}`,
    `sslcert=${gateway.certPath}`,
    `sslkey=${gateway.keyPath}`,
    `sslmode=${gateway.insecure ? SSL_MODE_VERIFY_CA : SSL_MODE_VERIFY_FULL}`,
    `user=[DB_USER]`,
    `dbname=[DB_NAME]`,
  ];

  const psqlConnStr = `psql "postgres://${gateway.localAddress}:${
    gateway.localPort
  }?${args.join('&')}"`;
  return psqlConnStr;
}

export type State = ReturnType<typeof usePostgres>;
