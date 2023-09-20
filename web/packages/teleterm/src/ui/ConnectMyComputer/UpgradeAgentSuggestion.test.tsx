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
    screen.findByText(/Consider upgrading it to 15.0.0/)
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
