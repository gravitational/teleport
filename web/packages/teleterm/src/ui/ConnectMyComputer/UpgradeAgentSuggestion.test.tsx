/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import { render } from 'design/utils/testing';
import { screen } from '@testing-library/react';

import {
  UpgradeAgentSuggestion,
  shouldShowAgentUpgradeSuggestion,
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
