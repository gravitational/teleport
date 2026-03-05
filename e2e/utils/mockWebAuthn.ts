/**
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

import { Page } from '@playwright/test';

// Private key for WebAuthn corresponding to the public key in state.yaml
const privateKeyBase64 =
  'MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgbW9ja3dlYmF1dGhuLXByaXZhdGUta2V5LWZvci1lMmWhRANCAAQeOk06HOqxNf941IwptLmIktnJcvFg6zIuRBZodcIV8/OnXPbY6w+6v188xvJe8rd5h5XbuskLq8zNNpmivCYu';

const credentialIdBase64 = 'ZTJlLXdlYmF1dGhuLWNyZWRlbnRpYWwtaWQtMDAwMQ==';

// mockWebAuthn sets up a virtual webauthn authenticator on the page.
export async function mockWebAuthn(page: Page) {
  const cdpSession = await page.context().newCDPSession(page);
  await cdpSession.send('WebAuthn.enable');

  const { authenticatorId } = await cdpSession.send(
    'WebAuthn.addVirtualAuthenticator',
    {
      options: {
        protocol: 'ctap2',
        transport: 'internal',
        hasResidentKey: true,
        hasUserVerification: true,
        isUserVerified: true,
      },
    }
  );

  await cdpSession.send('WebAuthn.addCredential', {
    authenticatorId,
    credential: {
      credentialId: credentialIdBase64,
      isResidentCredential: false,
      rpId: 'localhost',
      privateKey: privateKeyBase64,
      signCount: 0,
    },
  });

  const cleanup = async () => {
    await cdpSession.send('WebAuthn.removeVirtualAuthenticator', {
      authenticatorId,
    });
    await cdpSession.detach();
  };

  return { authenticatorId, cdpSession, cleanup };
}
