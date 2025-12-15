/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { screen } from '@testing-library/react';

import { render } from 'design/utils/testing';

import {
  shouldShowAgentUpgradeSuggestion,
  UpgradeAgentSuggestion,
} from './UpgradeAgentSuggestion';

test('upgradeAgentSuggestion renders correct versions', async () => {
  render(<UpgradeAgentSuggestion proxyVersion="15.0.0" appVersion="14.1.0" />);

  await expect(
    screen.findByText(/agent is running version 14.1.0 of Teleport/)
  ).resolves.toBeVisible();
  await expect(
    screen.findByText(/Consider upgrading it to version 15.0.0/)
  ).resolves.toBeVisible();
});

describe('shouldShowAgentUpgradeSuggestion returns', () => {
  const testCases = [
    {
      name: 'the agent is not compatible',
      isLocalBuild: false,
      appVersion: '15.0.0',
      proxyVersion: '17.0.0',
      expected: false,
    },
    {
      name: 'the agent is on a newer version',
      isLocalBuild: false,
      appVersion: '14.1.0',
      proxyVersion: '14.0.0',
      expected: false,
    },
    {
      name: 'the agent is on the same version',
      isLocalBuild: false,
      appVersion: '14.1.0',
      proxyVersion: '14.1.0',
      expected: false,
    },
    {
      name: 'is a dev build',
      isLocalBuild: true,
      appVersion: '1.0.0-dev',
      proxyVersion: '14.0.0',
      expected: false,
    },
    {
      name: 'the agent can be upgraded',
      isLocalBuild: false,
      appVersion: '14.1.0',
      proxyVersion: '14.2.0',
      expected: true,
    },
  ];
  test.each(testCases)(
    '$expected when $name',
    async ({ isLocalBuild, appVersion, proxyVersion, expected }) => {
      expect(
        shouldShowAgentUpgradeSuggestion(proxyVersion, {
          appVersion,
          isLocalBuild,
        })
      ).toBe(expected);
    }
  );
});
