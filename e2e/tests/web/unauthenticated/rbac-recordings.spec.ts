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

import { appendFileSync, cpSync, mkdirSync, readFileSync } from 'fs';
import { dirname, join } from 'path';
import { fileURLToPath } from 'url';

import { teleportConfig } from '@gravitational/e2e/helpers/env';
import { signup } from '@gravitational/e2e/helpers/signup';
import {
  createResource,
  deleteResource,
  deleteUser,
} from '@gravitational/e2e/helpers/tctl';
import { CLUSTER_NAME, expect, test } from '@gravitational/e2e/helpers/test';

const RECORDING_ID = 'e3286c06-d0ec-4d84-aeab-75692b885e3b';

const fixtureDir = join(
  dirname(fileURLToPath(import.meta.url)),
  '../../../config/fixtures/dummysessionrecording'
);

/**
 * injectRecording copies the session recording into the data dir and creates a session.end event for it
 */
function injectRecording(sessionId: string) {
  const configContent = readFileSync(teleportConfig, 'utf-8');
  const match = configContent.match(/data_dir:\s*(.+)/);
  if (!match) {
    throw new Error(`couldn't find data_dir in config: ${teleportConfig}`);
  }
  const dataDir = match[1].trim();

  // Copy recording files into the records dir
  const recordsDir = join(dataDir, 'log', 'records');
  mkdirSync(recordsDir, { recursive: true });
  cpSync(fixtureDir, recordsDir, { recursive: true });

  // Inject a session.end event into the audit log
  const now = new Date().toISOString();
  const sessionEnd = {
    ei: 1,
    event: 'session.end',
    uid: sessionId,
    code: 'T2004I',
    time: now,
    cluster_name: 'teleport-e2e',
    user: 'bob',
    login: 'root',
    sid: sessionId,
    namespace: 'default',
    server_id: 'e2e-server',
    server_hostname: 'teleport-e2e',
    server_addr: '[::]:3022',
    enhanced_recording: false,
    interactive: true,
    participants: ['bob'],
    session_start: now,
    session_stop: now,
    session_recording: 'node',
  };
  const eventsLog = join(dataDir, 'log', 'events.log');
  appendFileSync(eventsLog, JSON.stringify(sessionEnd) + '\n');
}

// Role with session list only
const sessionListOnlyRole = `kind: role
metadata:
  name: rbac-session-list
spec:
  allow:
    logins:
    - root
    node_labels:
      '*': '*'
    rules:
    - resources:
      - session
      verbs:
      - list
  options:
    max_session_ttl: 8h0m0s
version: v3`;

// Role with session list and read
const sessionReadRole = `kind: role
metadata:
  name: rbac-session-read
spec:
  allow:
    logins:
    - root
    node_labels:
      '*': '*'
    rules:
    - resources:
      - session
      verbs:
      - list
      - read
  options:
    max_session_ttl: 8h0m0s
version: v3`;

test('verify that playing a recorded session is denied without read access', async ({
  page,
}, testInfo) => {
  // We add a longer timeout to add some buffer because this includes a full signup flow and injecting a session recording.
  test.setTimeout(30_000);
  const username = `test-user-${testInfo.workerIndex}`;

  injectRecording(RECORDING_ID);
  createResource(sessionListOnlyRole);
  await signup(page, username, 'rbac-session-list');

  // Navigate to the session recordings page and verify that the recording is listed
  await page.goto(`/web/cluster/${CLUSTER_NAME}/recordings`);
  const recordingLink = page.getByTestId('recording-item').first();
  await expect(recordingLink).toBeVisible({ timeout: 15_000 });

  // Click on the recording and verify that there's an error
  const popupPromise = page.waitForEvent('popup');
  await recordingLink.click();
  const playerPage = await popupPromise;
  await playerPage.waitForLoadState('load');
  await expect(
    playerPage.getByText('unable to determine the length').first()
  ).toBeVisible({ timeout: 15_000 });

  deleteUser(username);
  deleteResource('role', 'rbac-session-list');
});

test('verify that a user can replay a session with read access', async ({
  page,
}, testInfo) => {
  // We add a longer timeout to add some buffer because this includes a full signup flow and injecting a session recording.
  test.setTimeout(30_000);
  const username = `test-user-${testInfo.workerIndex}`;

  injectRecording(RECORDING_ID);
  createResource(sessionReadRole);
  await signup(page, username, 'rbac-session-read');

  // Navigate to the session recordings page and verify that the recording is listed
  await page.goto(`/web/cluster/${CLUSTER_NAME}/recordings`);
  const recordingLink = page.getByTestId('recording-item').first();
  await expect(recordingLink).toBeVisible({ timeout: 15_000 });

  // Click on the recording and verify that the player shows up
  const popupPromise = page.waitForEvent('popup');
  await recordingLink.click();
  const playerPage = await popupPromise;
  await playerPage.waitForLoadState('load');
  await expect(playerPage.locator('.xterm')).toBeVisible({ timeout: 15_000 });

  deleteUser(username);
  deleteResource('role', 'rbac-session-read');
});
