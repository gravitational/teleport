// open-with-webauthn launches a Chromium browser with a virtual WebAuthn
// authenticator preloaded so that MFA challenges resolve automatically
// in codegen and browse modes.
//
// Usage: pnpm exec tsx scripts/open-with-webauthn.ts <codegen|open> <url>

import {
  chromium,
  firefox,
  webkit,
  type BrowserContext,
} from '@playwright/test';

import { authStateFor } from '../helpers/authState';
import { defaultUsername } from '../helpers/defaultUser';
import type { StorageState } from '../helpers/login';
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

const browserName = (process.env.E2E_BROWSERS || 'chromium').split(',')[0];
const browserTypes = { chromium, firefox, webkit };
const browserType =
  browserTypes[browserName as keyof typeof browserTypes] ?? chromium;
const username = defaultUsername();

// Runs against an existing cluster have no bootstrapped credentials to log in with, so open without a session
// rather than failing.
let storageState: StorageState | undefined;
if (process.env.E2E_USERS_FILE) {
  storageState = await authStateFor(username);
  info(`logged in as ${bold(username)}`);
}

info(`launching ${browserName} ${dim(`(mode: ${mode})`)}`);

const browser = await browserType.launch({ headless: false });
const context = await browser.newContext({
  storageState,
  ignoreHTTPSErrors: true,
  viewport: null,
});

const page = await context.newPage();

await mockWebAuthn(page, username);

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
