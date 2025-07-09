/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

// RoleEditorVisualizer is the component that will be shown
// next to the role editor adapter. This can either be the

import { useState } from 'react';

import { Alert } from 'design/Alert';
import Box from 'design/Box';
import Flex from 'design/Flex';
import { ShieldCheck } from 'design/Icon';

import cfg from 'teleport/config';
import { getSalesURL } from 'teleport/services/sales';
import useTeleport from 'teleport/useTeleport';

import { PolicyPlaceholder } from '../PolicyPlaceholder';
import { RoleDiffProps, RoleDiffState } from '../Roles';

// role diff visualizer, or the upsell
export function RoleEditorVisualizer({
  roleDiffProps,
  currentFlow,
}: {
  roleDiffProps?: RoleDiffProps;
  currentFlow: 'creating' | 'updating';
}) {
  const ctx = useTeleport();
  const version = ctx.storeUser.state.cluster.authVersion;
  const canUpdateAccessGraphSettings =
    ctx.storeUser.state.acl.accessGraphSettings.edit &&
    cfg.entitlements.AccessGraphDemoMode.enabled;
  // the demo banner should show every time they load the role editor
  const [demoDismissed, setDemoDismissed] = useState(false);
  if (roleDiffProps && shouldShowRoleDiff(roleDiffProps)) {
    return (
      <Flex
        flex="1"
        flexDirection="column"
        css={`
          position: relative;
        `}
      >
        <Box
          data-testid="demo-banner"
          css={`
            position: absolute;
            width: 100%;
            z-index: 1000;
            padding: 20px;
          `}
        >
          {!demoDismissed &&
            roleDiffProps.roleDiffState === RoleDiffState.DemoReady && (
              <Alert
                kind="neutral"
                details="Secure identities and access policies across all of your infrastructure. Eliminate shadow access and blind spots."
                icon={ShieldCheck}
                primaryAction={{
                  content: 'Contact Sales',
                  href: getSalesURL(version, false),
                }}
                secondaryAction={{
                  content: 'Learn More',
                  href: 'https://goteleport.com/platform/policy/',
                }}
                dismissible
                onDismiss={() => setDemoDismissed(true)}
              >
                Teleport Identity Security
              </Alert>
            )}
        </Box>
        {roleDiffProps.roleDiffElement}
      </Flex>
    );
  }

  return (
    <Flex flex="1" alignItems="center" justifyContent="center" m={3}>
      <PolicyPlaceholder
        canUpdateAccessGraphSettings={canUpdateAccessGraphSettings}
        roleDiffProps={roleDiffProps}
        enableDemoMode={roleDiffProps?.enableDemoMode}
        currentFlow={currentFlow}
      />
    </Flex>
  );
}

export function shouldShowRoleDiff(rdp: RoleDiffProps) {
  return (
    rdp.roleDiffState === RoleDiffState.DemoReady ||
    rdp.roleDiffState === RoleDiffState.PolicyEnabled
  );
}
