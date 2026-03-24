/**
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

import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

import {
  expect,
  initializeDataDir,
  launchApp,
  login,
  test,
  withDefaultAppConfig,
} from '@gravitational/e2e/helpers/connect';
import { leafTeleportConfig, startUrl } from '@gravitational/e2e/helpers/env';
import { tctl, tctlCreate } from '@gravitational/e2e/helpers/tctl';

const roleDefinitions = [
  `kind: role
metadata:
  name: allow-roles-and-nodes
spec:
  allow:
    logins: [root]
    node_labels:
      '*': '*'
    rules:
      - resources: [role]
        verbs: [list, read]
  options:
    max_session_ttl: 8h0m0s
version: v5`,
  `kind: role
metadata:
  name: allow-users-with-short-ttl
spec:
  allow:
    rules:
      - resources: [user]
        verbs: [list, read]
  deny:
    node_labels:
      '*': '*'
  options:
    max_session_ttl: 4m0s
version: v5`,
  `kind: role
metadata:
  name: test-role-based-requests
spec:
  allow:
    request:
      roles:
        - allow-roles-and-nodes
        - allow-users-with-short-ttl
      suggested_reviewers:
        - bob
version: v5`,
  `kind: role
metadata:
  name: reviewer
spec:
  allow:
    review_requests:
      roles: ['*']
version: v3`,
  `kind: role
metadata:
  name: searcheable-resources
spec:
  allow:
    logins: [root]
    node_labels:
      '*': '*'
version: v5`,
  `kind: role
metadata:
  name: test-search-based-requests
spec:
  allow:
    request:
      search_as_roles:
        - searcheable-resources
      suggested_reviewers:
        - bob
version: v5`,
];

const trustedClusterWithTestRoles = `kind: trusted_cluster
version: v2
metadata:
  name: teleport-e2e
spec:
  enabled: true
  token: foo
  web_proxy_addr: ${new URL(startUrl).host}
  role_map:
    - remote: access
      local: [access]
    - remote: editor
      local: [editor]
    - remote: test-role-based-requests
      local: [test-role-based-requests]
    - remote: test-search-based-requests
      local: [test-search-based-requests]
    - remote: searcheable-resources
      local: [searcheable-resources]
    - remote: reviewer
      local: [reviewer]`;

function getUserRoles(username: string): string[] {
  const output = tctl('get', `user/${username}`, '--format=json');
  const users = JSON.parse(output);
  return users[0]?.spec?.roles ?? [];
}

function deleteAllAccessRequests() {
  const output = tctl('requests', 'ls', '--format=json').trim();
  const requests = JSON.parse(output);
  for (const req of requests) {
    tctl('requests', 'rm', req.metadata.name, '--force');
  }
}

test.describe('access requests', () => {
  test.describe.configure({ mode: 'serial' });
  test.skip(
    !process.env.E2E_ACCESS_REQUESTS,
    'requires enterprise cluster with leaf cluster'
  );

  let requesterSnapshot: string;
  let reviewerSnapshot: string;
  let originalAliceRoles: string[];
  let originalBobRoles: string[];

  test.beforeAll(async () => {
    // Create roles on both root and leaf clusters.
    for (const role of roleDefinitions) {
      tctlCreate(role);
      tctlCreate(role, { config: leafTeleportConfig });
    }

    // Update the trusted cluster role mapping to include the new roles.
    tctlCreate(trustedClusterWithTestRoles, { config: leafTeleportConfig });

    originalAliceRoles = getUserRoles('alice');
    originalBobRoles = getUserRoles('bob');

    tctl(
      'users',
      'update',
      'alice',
      '--set-roles=test-role-based-requests,test-search-based-requests'
    );
    tctl('users', 'update', 'bob', '--set-roles=access,editor,reviewer');

    requesterSnapshot = await fs.mkdtemp(
      path.join(os.tmpdir(), 'connect-e2e-requester-')
    );
    await initializeDataDir(requesterSnapshot, withDefaultAppConfig({}));
    {
      await using app = await launchApp(requesterSnapshot);
      await login(app.page, 'alice');
    }

    reviewerSnapshot = await fs.mkdtemp(
      path.join(os.tmpdir(), 'connect-e2e-reviewer-')
    );
    await initializeDataDir(reviewerSnapshot, withDefaultAppConfig({}));
    {
      await using app = await launchApp(reviewerSnapshot);
      await login(app.page, 'bob');
    }
  });

  test.afterEach(() => {
    deleteAllAccessRequests();
  });

  test.afterAll(async () => {
    tctl(
      'users',
      'update',
      'alice',
      `--set-roles=${originalAliceRoles.join(',')}`
    );
    tctl('users', 'update', 'bob', `--set-roles=${originalBobRoles.join(',')}`);

    await fs.rm(requesterSnapshot, { recursive: true, force: true });
    await fs.rm(reviewerSnapshot, { recursive: true, force: true });
  });

  async function launchFromSnapshot(snapshot: string) {
    await using stack = new AsyncDisposableStack();
    const temp = stack.use(
      await fs.mkdtempDisposable(path.join(os.tmpdir(), 'connect-e2e-ar-'))
    );
    await fs.cp(snapshot, temp.path, { recursive: true });
    const launched = stack.use(await launchApp(temp.path));
    const disposables = stack.move();
    return {
      ...launched,
      [Symbol.asyncDispose]: () => disposables.disposeAsync(),
    };
  }

  async function launchAsRequester() {
    return launchFromSnapshot(requesterSnapshot);
  }

  // oxlint-disable-next-line eslint/no-unused-vars
  async function launchAsReviewer() {
    return launchFromSnapshot(reviewerSnapshot);
  }

  test('leaf cluster is visible with resources', async () => {
    await using app = await launchAsRequester();
    const { page } = app;

    const clusterSelector = page.locator('[title*="Open Clusters"]');
    await expect(clusterSelector).toBeVisible();

    await clusterSelector.click();
    await page.getByText('teleport-e2e-leaf').click();

    await expect(page.getByText('docker-leaf-node')).toBeVisible();
  });
});
