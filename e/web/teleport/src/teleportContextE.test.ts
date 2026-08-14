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

import { UserPreferences } from 'gen-proto-ts/teleport/userpreferences/v1/userpreferences_pb';

import cfg from 'e-teleport/config';
import TeleportEContext from 'e-teleport/teleportContextE';
import { getAcl, getUserContext } from 'teleport/mocks/contexts';
import userService from 'teleport/services/user';

const preferences = {} as UserPreferences;

let originalEntitlements: typeof cfg.oss.entitlements;
let originalBeamsUi: boolean;

beforeEach(() => {
  originalEntitlements = structuredClone(cfg.oss.entitlements);
  originalBeamsUi = cfg.oss.beamsUi;

  cfg.oss.entitlements.Beams.enabled = true;
  cfg.oss.beamsUi = true;
});

afterEach(() => {
  cfg.oss.entitlements = originalEntitlements;
  cfg.oss.beamsUi = originalBeamsUi;

  jest.restoreAllMocks();
});

async function initContext({ beamAccess }: { beamAccess: boolean }) {
  const user = getUserContext();
  user.acl = getAcl({ noAccess: true });
  user.acl.beam = {
    list: beamAccess,
    read: beamAccess,
    create: false,
    edit: false,
    remove: false,
  };

  jest.spyOn(userService, 'fetchUserContext').mockResolvedValue(user);

  const ctx = new TeleportEContext();
  await ctx.init(preferences);

  return ctx.redirectUrl;
}

describe('beams quickstart redirect', () => {
  test('redirects a user with beam permissions', async () => {
    cfg.oss.entitlements.FeatureHiding.enabled = true;

    await expect(initContext({ beamAccess: true })).resolves.toBe(
      cfg.getBeamsQuickstartRoute()
    );
  });

  test('redirects a user without beam permissions when feature hiding is off', async () => {
    cfg.oss.entitlements.FeatureHiding.enabled = false;

    await expect(initContext({ beamAccess: false })).resolves.toBe(
      cfg.getBeamsQuickstartRoute()
    );
  });

  test('does not redirect a user without beam permissions when feature hiding is on', async () => {
    cfg.oss.entitlements.FeatureHiding.enabled = true;

    await expect(initContext({ beamAccess: false })).resolves.toBeNull();
  });

  test('does not redirect when the beams UI is disabled', async () => {
    cfg.oss.beamsUi = false;

    await expect(initContext({ beamAccess: true })).resolves.toBeNull();
  });

  test('does not redirect without the beams entitlement', async () => {
    cfg.oss.entitlements.Beams.enabled = false;

    await expect(initContext({ beamAccess: true })).resolves.toBeNull();
  });
});
