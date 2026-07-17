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

import { randomUUID } from 'node:crypto';

import { deleteResourceIfExists } from '@gravitational/e2e/helpers/tctl';
import { expect, test } from '@gravitational/e2e/helpers/test';
import type { Page, Request } from '@playwright/test';

const MFA_CHALLENGE_PATH = '/v1/webapi/mfa/authenticatechallenge';
const SET_DEFAULT_PATH = '/v1/webapi/authconnector/default';

const isMfaChallenge = (req: Request) => req.url().includes(MFA_CHALLENGE_PATH);

// TODO(@rudream): re-enable once #67593 (auth connector MFA papercuts) is
// backported. Until then v18 still requires MFA to list connectors.
test.skip('lists, creates, views, and deletes a connector', async ({
  page,
}) => {
  const name = `testconnector-${randomUUID()}`;
  const tile = page.getByTestId(`${name}-tile`);

  try {
    await test.step('listing does not require MFA', async () => {
      await expectNoMfaChallenge(page, async () => {
        await page.goto('/web/sso');
        await expect(
          page.getByRole('heading', { name: 'Your Connectors' })
        ).toBeVisible();
      });
    });

    await test.step('creating requires MFA', () => createConnector(page, name));

    await test.step('viewing renders secrets', async () => {
      await tile.getByRole('button').click();
      await expectMfaChallenge(page, () =>
        page.getByRole('menuitem', { name: 'Edit' }).click()
      );

      await page.waitForSelector('.ace_editor', { state: 'visible' });
      await expect.poll(() => getAceValue(page)).toContain('<client-secret>');
    });

    await test.step('deleting requires MFA', async () => {
      await page.goto('/web/sso');
      await tile.getByRole('button').click();
      await page.getByRole('menuitem', { name: 'Delete' }).click();

      await expectMfaChallenge(page, () =>
        page.getByRole('button', { name: 'Delete Connector' }).click()
      );
      await expect(tile).not.toBeVisible();
    });
  } finally {
    deleteResourceIfExists(`github/${name}`);
  }
});

// TODO(@rudream): re-enable once #67593 (auth connector MFA papercuts) is
// backported. The fallback path depends on the reusable MFA response it adds.
test.describe.skip('default connector fallback', () => {
  const name = `testconnector-${randomUUID()}`;

  test.afterEach(async ({ page }) => {
    deleteResourceIfExists(`github/${name}`);
    await expect(async () => {
      await page.goto('/web/sso');
      await expect(
        page.getByRole('heading', { name: 'Your Connectors' })
      ).toBeVisible();
    }).toPass();
  });

  test('if the default connector was deleted, the fallback works and prompts for mfa', async ({
    page,
  }) => {
    await createConnector(page, name);
    await setAsDefault(page, name);

    // Delete the connector via tctl
    deleteResourceIfExists(`github/${name}`);

    // Loading the connectors page should prompt for MFA, since in the background the default connector is being
    // set to a fallback connector, which is an admin action.
    await expect(async () => {
      const challenge = page.waitForRequest(isMfaChallenge, { timeout: 3000 });
      await page.goto('/web/sso');
      await challenge;
    }).toPass();
  });
});

// createConnector creates an auth connector
async function createConnector(page: Page, name: string) {
  await page.goto('/web/sso');
  await page.getByRole('button', { name: 'New GitHub Connector' }).click();
  await page.waitForSelector('.ace_editor', { state: 'visible' });
  await setConnectorName(page, name);

  await expectMfaChallenge(page, () =>
    page.getByRole('button', { name: 'Save Changes' }).click()
  );
  await expect(page.getByTestId(`${name}-tile`)).toBeVisible();
}

// setAsDefault sets a connector as default
async function setAsDefault(page: Page, name: string) {
  await page.getByTestId(`${name}-tile`).getByRole('button').click();
  await Promise.all([
    page.waitForResponse(r => r.url().includes(SET_DEFAULT_PATH) && r.ok()),
    page.getByRole('menuitem', { name: 'Set as default' }).click(),
  ]);
}

// expectMfaChallenge verifies that `action` prompts for MFA.
async function expectMfaChallenge(page: Page, action: () => Promise<void>) {
  await Promise.all([page.waitForRequest(isMfaChallenge), action()]);
}

// expectNoMfaChallenge verifies that `action` does not prompt for MFA.
async function expectNoMfaChallenge(page: Page, action: () => Promise<void>) {
  let prompted = false;
  const listener = (req: Request) => {
    if (isMfaChallenge(req)) prompted = true;
  };
  page.on('request', listener);
  try {
    await action();
  } finally {
    page.off('request', listener);
  }
  expect(prompted, 'expected no MFA challenge').toBe(false);
}

// setConnectorName sets the name field in the new GitHub connector editor.
async function setConnectorName(page: Page, name: string) {
  await page.evaluate(name => {
    const editor = (window as any).ace.edit(
      document.querySelector('.ace_editor')
    );
    const lines = editor.session.getDocument().getAllLines();
    lines[3] = `  name: ${name}`;
    editor.session.setValue(lines.join('\n'));
  }, name);
}

// getAceValue returns the text content of the visible ACE editor.
function getAceValue(page: Page): Promise<string> {
  return page.evaluate(
    () =>
      (window as any).ace
        .edit(document.querySelector('.ace_editor'))
        .session.getValue() as string
  );
}
