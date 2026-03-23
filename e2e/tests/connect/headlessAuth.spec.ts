/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

import { test, expect } from '@gravitational/e2e/helpers/connect';
import { connectTshBin, startUrl } from '@gravitational/e2e/helpers/env';
import { login as webLogin } from '@gravitational/e2e/helpers/login';
import { HeadlessAuthDialogPage } from '@gravitational/e2e/helpers/pages/connect/HeadlessAuthDialog';
import { chromium } from '@playwright/test';

const REQUEST_URL_RE = /https?:\/\/\S+\/web\/headless\/[0-9a-f-]+/i;
const HEADLESS_USER = 'bob';

type CreatedRequest = {
  url: string;
  id: string;
};

type HeadlessRequestProcess = {
  request: CreatedRequest;
  // Throws if process exits with non-zero exit code.
  waitForExit(): Promise<void>;
  [Symbol.asyncDispose](): Promise<void>;
};

async function startHeadlessRequestProcess(options?: {
  abortSignal?: AbortSignal;
}): Promise<HeadlessRequestProcess> {
  const proxyHost = new URL(startUrl).host;

  await using disposer = new AsyncDisposableStack();
  const homeDir = disposer.use(
    await fs.mkdtempDisposable(path.join(os.tmpdir(), 'headless-e2e-'))
  );
  const child: ChildProcessWithoutNullStreams = disposer.use(
    spawn(
      connectTshBin,
      [
        'ls',
        '--headless',
        '--insecure',
        `--user=${HEADLESS_USER}`,
        `--proxy=${proxyHost}`,
      ],
      {
        env: {
          ...process.env,
          TELEPORT_HOME: homeDir.path,
        },
        signal: options?.abortSignal,
        stdio: ['pipe', 'pipe', 'pipe'],
      }
    )
  );

  let output = '';
  const requestCreated = Promise.withResolvers<CreatedRequest>();
  const exited = Promise.withResolvers<void>();

  child.stderr.on('data', chunk => {
    output += chunk.toString('utf8');
    const match = output.match(REQUEST_URL_RE);
    if (match) {
      const url = match[0];
      const id = new URL(url).pathname.split('/').at(-1);
      if (!id) {
        requestCreated.reject(
          new Error(`could not parse request id from URL: ${url}`)
        );
        return;
      }
      requestCreated.resolve({ url, id });
    }
  });

  child.once('exit', code => {
    if (code !== 0) {
      exited.reject(new Error(`Process exited with non zero code: ${output}`));
      return;
    }
    exited.resolve();
  });

  child.once('error', error => {
    if (error.name === 'AbortError') {
      exited.resolve();
      return;
    }
    exited.reject(error);
  });

  const request = await Promise.race([
    requestCreated.promise,
    exited.promise.then(() => {
      throw new Error(
        `Process exited before creating request. Output:\n${output}`
      );
    }),
  ]);

  const disposables = disposer.move();
  return {
    request,
    waitForExit: () => exited.promise,
    [Symbol.asyncDispose]: () => disposables.disposeAsync(),
  };
}

async function approveInWebUi(requestUrl: string) {
  const browser = await chromium.launch();
  const context = await browser.newContext({
    baseURL: startUrl,
    ignoreHTTPSErrors: true,
  });
  const page = await context.newPage();

  try {
    await webLogin(page);
    await page.goto(requestUrl);

    await expect(page.getByRole('button', { name: 'Approve' })).toBeVisible();
    await page.getByRole('button', { name: 'Approve' }).click();

    const passkeyButton = page.getByRole('button', {
      name: 'Passkey or MFA Device',
    });
    await passkeyButton.click();
  } finally {
    await context.close();
    await browser.close();
  }
}

test.use({ autoLogin: true });
test.setTimeout(30_000);

test('headless auth modal flows', async ({ app }) => {
  const { page } = app;
  const headlessDialog = new HeadlessAuthDialogPage(page);

  await test.step('approve, reject, and ignore then approve in Web UI', async () => {
    await using process1 = await startHeadlessRequestProcess();
    await headlessDialog.approve();
    await headlessDialog.waitForClose();
    await expect(process1.waitForExit()).resolves.toBeUndefined();

    await using process2 = await startHeadlessRequestProcess();
    const exitRejected = expect(process2.waitForExit()).rejects.toThrow(
      /headless authentication denied/i
    );
    await headlessDialog.reject();
    await headlessDialog.waitForClose();
    await exitRejected;

    await using process3 = await startHeadlessRequestProcess();
    await headlessDialog.close();
    await headlessDialog.waitForClose();
    await approveInWebUi(process3.request.url);
    await expect(process3.waitForExit()).resolves.toBeUndefined();
  });

  await test.step('canceling a headless command closes the modal automatically', async () => {
    const abortController = new AbortController();
    await using process = await startHeadlessRequestProcess({
      abortSignal: abortController.signal,
    });
    await headlessDialog.waitForVisible();

    abortController.abort();
    await headlessDialog.waitForClose();
    await expect(process.waitForExit()).resolves.toBeUndefined();
  });

  await test.step('approving a headless command in Web UI closes the modal automatically', async () => {
    const abortController = new AbortController();
    await using process = await startHeadlessRequestProcess({
      abortSignal: abortController.signal,
    });
    await headlessDialog.waitForVisible();

    await approveInWebUi(process.request.url);
    await expect(process.waitForExit()).resolves.toBeUndefined();
    await headlessDialog.waitForClose();
  });

  await test.step('shows the second concurrent request after closing the first modal', async () => {
    await using process1 = await startHeadlessRequestProcess();
    await using process2 = await startHeadlessRequestProcess();

    const requestId1 = process1.request.id;
    const requestId2 = process2.request.id;
    const requestsById: Record<string, HeadlessRequestProcess> = {
      [requestId1]: process1,
      [requestId2]: process2,
    };

    const firstVisibleRequestId = await Promise.any([
      headlessDialog.waitForRequestId(requestId1).then(() => requestId1),
      headlessDialog.waitForRequestId(requestId2).then(() => requestId2),
    ]);
    const secondVisibleRequestId =
      firstVisibleRequestId === requestId1 ? requestId2 : requestId1;

    await headlessDialog.approve();
    await headlessDialog.waitForClose();
    await requestsById[firstVisibleRequestId].waitForExit();

    await headlessDialog.waitForRequestId(secondVisibleRequestId);
    await headlessDialog.approve();
    await headlessDialog.waitForClose();
    await requestsById[secondVisibleRequestId].waitForExit();
  });
});
