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

// open-with-webauthn launches a Chromium browser with a virtual WebAuthn
// authenticator preloaded so that MFA challenges resolve automatically
// in codegen and browse modes.
//
// Usage: pnpm exec tsx scripts/open-with-webauthn.ts <codegen|open> <url>

import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

import {
  chromium,
  firefox,
  webkit,
  type BrowserContext,
} from '@playwright/test';

import { mockWebAuthn } from '../helpers/webauthn';

const bold = (s: string) => `\x1b[1m${s}\x1b[22m`;
const green = (s: string) => `\x1b[32m${s}\x1b[39m`;
const cyan = (s: string) => `\x1b[36m${s}\x1b[39m`;
const red = (s: string) => `\x1b[31m${s}\x1b[39m`;
const dim = (s: string) => `\x1b[2m${s}\x1b[22m`;

function info(msg: string) {
  process.stdout.write(`${green('✓')} ${msg}\n`);
}

function error(msg: string) {
  process.stderr.write(`${red('✗')} ${msg}\n`);
}

const mode = process.argv[2] as 'codegen' | 'open';
const startURL = process.argv[3];

if (!mode || !startURL) {
  error(
    'Usage: pnpm exec tsx scripts/open-with-webauthn.ts <codegen|open> <url>'
  );
  process.exit(1);
}

const e2eDir = join(dirname(fileURLToPath(import.meta.url)), '..');
const browserName = (process.env.E2E_BROWSERS || 'chromium').split(',')[0];
const browserTypes = { chromium, firefox, webkit };
const browserType =
  browserTypes[browserName as keyof typeof browserTypes] ?? chromium;
const storageStatePath = join(e2eDir, `.auth/${browserName}-user.json`);

info(`launching ${browserName} ${dim(`(mode: ${mode})`)}`);

const browser = await browserType.launch({ headless: false });
const context = await browser.newContext({
  storageState: storageStatePath,
  ignoreHTTPSErrors: true,
});

const page = await context.newPage();

await mockWebAuthn(page);

info('virtual WebAuthn authenticator registered');

if (mode === 'codegen') {
  // _enableRecorder is the internal API that `playwright codegen` itself uses
  // to attach the code-generation inspector to a browser context.
  type ExtendedBrowserContext = BrowserContext & {
    _enableRecorder: (options: {
      language: string;
      mode: 'recording';
    }) => Promise<void>;
  };

  await (context as ExtendedBrowserContext)._enableRecorder({
    language: 'javascript',
    mode: 'recording',
  });

  info('Playwright recorder enabled');
}

info(`navigating to ${cyan(bold(startURL))}`);

await page.goto(startURL);

// Exit when the user closes the last page or the browser disconnects.
page.on('close', () => {
  if (context.pages().length === 0) {
    browser.close().finally(() => process.exit(0));
  }
});
browser.on('disconnected', () => process.exit(0));

// Block until exit.
await new Promise(() => {});
