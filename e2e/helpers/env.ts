/**
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

// This file provides helper functions to read required environment variables for the E2E tests.

function required(name: string) {
  const value = process.env[name];

  if (!value) {
    throw new Error(`required environment variable ${name} is not set`);
  }

  return value;
}

export const password = required('E2E_PASSWORD');
export const webauthnPrivateKey = required('E2E_WEBAUTHN_PRIVATE_KEY');
export const webauthnCredentialId = required('E2E_WEBAUTHN_CREDENTIAL_ID');
export const tctlBin = required('E2E_TCTL_BIN');
export const teleportConfig = required('E2E_TELEPORT_CONFIG');
export const startUrl = required('START_URL');
export const connectTshBin = required('E2E_CONNECT_TSH_BIN');
export const connectAppDir = required('E2E_CONNECT_APP_DIR');
export const leafProxyUrl = process.env.E2E_LEAF_PROXY_URL || '';
