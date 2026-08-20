// This file provides helper functions to read required environment variables for the E2E tests.

import { readFileSync } from 'node:fs';

function required(name: string) {
  const value = process.env[name];

  if (!value) {
    throw new Error(`required environment variable ${name} is not set`);
  }

  return value;
}

export type UserCredentials = {
  password: string;
  webauthnPrivateKey: string;
  webauthnCredentialId: string;
  // Assigned by the runner so each user's login gets its own rate limiter bucket.
  clientIp: string;
};

export const users: Record<string, UserCredentials> = JSON.parse(
  readFileSync(required('E2E_USERS_FILE'), 'utf-8')
);
export const tctlBin = required('E2E_TCTL_BIN');
export const teleportConfig = required('E2E_TELEPORT_CONFIG');
export const startUrl = required('START_URL');
export const connectTshBin = required('E2E_CONNECT_TSH_BIN');
export const connectAppDir = required('E2E_CONNECT_APP_DIR');

// Set by the runner when the docker daemon's kernel cannot load the enhanced recording programs.
/** @public Documented in e2e/README.md for tests that need to skip on such daemons. */
export const skipEnhancedRecording =
  process.env.E2E_SKIP_ENHANCED_RECORDING === '1';
