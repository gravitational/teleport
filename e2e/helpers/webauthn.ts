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

import { Page } from '@playwright/test';

const privateKeyBase64 = process.env.E2E_WEBAUTHN_PRIVATE_KEY;
const credentialIdBase64 = process.env.E2E_WEBAUTHN_CREDENTIAL_ID;

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

  return { authenticatorId, cdpSession };
}
