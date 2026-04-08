/*
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

import { expect, type Page } from '@playwright/test';

import { users } from './env';
import { mockWebAuthn, signWebAuthnAssertion } from './webauthn';

export async function login(page: Page, username: string, password?: string) {
  if (!password) {
    password = users[username]?.password;

    if (!password) {
      throw new Error(`no credentials found for user "${username}"`);
    }
  }

  await page.addInitScript(() => {
    localStorage.setItem('grv_teleport_license_acknowledged', 'true');
    localStorage.setItem(
      'grv_teleport_identity_security_recommendations_unified_resources_cta_seen',
      'true'
    );
  });

  await mockWebAuthn(page, username);

  await page.goto('/');

  await page.getByPlaceholder('Username').fill(username);
  await page.getByPlaceholder('Password').fill(password);

  await page
    .getByTestId('userpassword')
    .getByRole('button', { name: 'Sign In' })
    .click();

  await page.waitForLoadState('networkidle');

  await expect(page.getByText(/^Resources$/).first()).toBeVisible();
}

export async function logout(page: Page) {
  await page.getByRole('button', { name: 'User Menu' }).click();
  await page.getByRole('menuitem', { name: 'Logout' }).click();
  // This is important to make sure that the redirect and subsequent page.goto
  // inside login() don't enter a race condition which results in a
  // net::ERR_ABORTED.
  await expect(page.getByText('Sign in to Teleport')).toBeVisible();
}

// StorageState matches the format that Playwright's BrowserContext.storageState()
// produces and that its `storageState` option consumes.
export interface StorageState {
  cookies: StorageCookie[];
  origins: StorageOrigin[];
}

interface StorageCookie {
  name: string;
  value: string;
  domain: string;
  path: string;
  expires: number; // unix seconds, -1 for session cookies
  httpOnly: boolean;
  secure: boolean;
  sameSite: 'Strict' | 'Lax' | 'None';
}

interface StorageOrigin {
  origin: string;
  localStorage: { name: string; value: string }[];
}

interface LoginBeginResponse {
  webauthn_challenge: {
    publicKey: {
      challenge: string; // base64url
      rpId: string;
      allowCredentials?: { type: string; id: string }[];
    };
  };
}

interface LoginFinishResponse {
  type: string;
  token: string;
  expires_in: number;
  sessionExpires: string;
  sessionExpiresIn: number;
  sessionInactiveTimeout: number;
}

/**
 * performHttpLogin replicates the web-UI login ceremony over HTTP so we don't
 * need to drive a real browser to produce a logged-in storage state. The
 * resulting StorageState can be written to disk and passed to Playwright's
 * `storageState` option.
 */
export async function directLogin(
  startUrl: string,
  username: string,
  password: string
): Promise<StorageState> {
  const url = new URL(startUrl);

  const beginRes = await fetch(
    new URL('/v1/webapi/mfa/login/begin', url).toString(),
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        passwordless: false,
        user: username,
        pass: password,
      }),
    }
  );

  if (!beginRes.ok) {
    throw new Error(
      `mfa/login/begin failed: ${beginRes.status} ${await beginRes.text()}`
    );
  }

  const challenge = (await beginRes.json()) as LoginBeginResponse;

  if (!challenge.webauthn_challenge?.publicKey) {
    throw new Error('login challenge response missing webauthn_challenge');
  }

  const assertion = signWebAuthnAssertion(
    username,
    challenge.webauthn_challenge.publicKey.challenge,
    challenge.webauthn_challenge.publicKey.rpId,
    url.origin
  );

  const finishRes = await fetch(
    new URL('/v1/webapi/mfa/login/finishsession', url).toString(),
    {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        user: username,
        webauthnAssertionResponse: assertion,
      }),
    }
  );

  if (!finishRes.ok) {
    throw new Error(
      `mfa/login/finishsession failed: ${finishRes.status} ${await finishRes.text()}`
    );
  }

  const session = (await finishRes.json()) as LoginFinishResponse;
  const cookies = parseSetCookieHeaders(finishRes.headers, url.hostname);

  if (!cookies.some(c => c.name === '__Host-session')) {
    throw new Error('login finish response did not set __Host-session cookie');
  }

  const nowMs = Date.now();
  const localStorage: { name: string; value: string }[] = [
    {
      name: 'grv_teleport_token',
      value: JSON.stringify({
        accessToken: session.token,
        expiresIn: session.expires_in,
        created: nowMs,
        sessionExpires: session.sessionExpires,
        sessionExpiresIn: session.sessionExpiresIn,
        sessionInactiveTimeout: session.sessionInactiveTimeout,
      }),
    },
    {
      name: 'grv_teleport_login_time',
      value: String(nowMs),
    },
    {
      name: 'grv_teleport_license_acknowledged',
      value: 'true',
    },
    {
      name: 'grv_teleport_identity_security_recommendations_unified_resources_cta_seen',
      value: 'true',
    },
  ];

  if (session.sessionInactiveTimeout) {
    localStorage.push({
      name: 'grv_teleport_last_active',
      value: String(nowMs + session.sessionInactiveTimeout),
    });
  }

  return {
    cookies,
    origins: [{ origin: url.origin, localStorage }],
  };
}

function parseSetCookieHeaders(
  headers: Headers,
  defaultDomain: string
): StorageCookie[] {
  return headers.getSetCookie().map(h => parseSetCookie(h, defaultDomain));
}

function parseSetCookie(header: string, defaultDomain: string): StorageCookie {
  const [nameVal, ...attrs] = header.split(';').map(s => s.trim());

  const eq = nameVal.indexOf('=');
  const name = nameVal.slice(0, eq);
  const value = nameVal.slice(eq + 1);

  const cookie: StorageCookie = {
    name,
    value,
    domain: defaultDomain,
    path: '/',
    expires: -1,
    httpOnly: false,
    secure: false,
    sameSite: 'Lax',
  };

  for (const attr of attrs) {
    const [k, ...rest] = attr.split('=');
    const key = k.toLowerCase();
    const v = rest.join('=');
    switch (key) {
      case 'domain':
        cookie.domain = v;
        break;
      case 'path':
        cookie.path = v;
        break;
      case 'expires':
        cookie.expires = Math.floor(new Date(v).getTime() / 1000);
        break;
      case 'max-age':
        cookie.expires = Math.floor(Date.now() / 1000) + Number(v);
        break;
      case 'httponly':
        cookie.httpOnly = true;
        break;
      case 'secure':
        cookie.secure = true;
        break;
      case 'samesite':
        cookie.sameSite = (v.charAt(0).toUpperCase() +
          v.slice(1).toLowerCase()) as StorageCookie['sameSite'];
        break;
    }
  }

  return cookie;
}
