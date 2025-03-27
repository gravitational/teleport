/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useCallback, useEffect, useState } from 'react';
import { useTheme } from 'styled-components';

import { Alert, Danger } from 'design/Alert';
import Box from 'design/Box';
import Flex from 'design/Flex';
import { ShieldCheck } from 'design/Icon';
import { Indicator } from 'design/Indicator';
import { useAsync } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import { State as ResourcesState } from 'teleport/components/useResources';
import { Role, RoleWithYaml } from 'teleport/services/resources';
import { getSalesURL } from 'teleport/services/sales';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';
import useTeleport from 'teleport/useTeleport';

import { PolicyPlaceholder } from '../PolicyPlaceholder';
import { RoleDiffProps, RoleDiffState, RolesProps } from '../Roles';
import { RoleEditor } from './RoleEditor';

/**
 * This component is responsible for converting from the `Resource`
 * representation of a role to a more accurate `RoleWithYaml` structure. The
 * conversion is asynchronous and it's performed on the server side.
 */
export function RoleEditorAdapter({
  resources,
  onSave,
  onCancel,
  roleDiffProps,
}: {
  resources: ResourcesState;
  onSave: (role: Partial<RoleWithYaml>) => Promise<void>;
  onCancel: () => void;
} & RolesProps) {
  const theme = useTheme();
  const [convertAttempt, convertToRole] = useAsync(
    async (yaml: string): Promise<RoleWithYaml | null> => {
      if (resources.status === 'creating' || !resources.item) {
        return null;
      }
      return {
        yaml,
        object: await yamlService.parse<Role>(YamlSupportedResourceKind.Role, {
          yaml,
        }),
      };
    }
  );

  const originalContent = resources.item?.content ?? '';
  useEffect(() => {
    convertToRole(originalContent);
  }, [originalContent]);

  const onRoleUpdate = useCallback(
    debounce(role => roleDiffProps?.updateRoleDiff(role), 500),
    []
  );

  return (
    <Flex flex="1">
      <Flex
        flexDirection="column"
        borderLeft={1}
        borderColor={theme.colors.interactive.tonal.neutral[0]}
        backgroundColor={theme.colors.levels.surface}
        width="550px"
      >
        {convertAttempt.status === 'processing' && (
          <Flex
            flexDirection="column"
            alignItems="center"
            justifyContent="center"
            flex="1"
          >
            <Indicator />
          </Flex>
        )}
        {convertAttempt.status === 'error' && (
          <Danger>{convertAttempt.statusText}</Danger>
        )}

        {/* TODO(bl-nero): Remove once RoleE doesn't set this attribute. */}
        {roleDiffProps?.errorMessage && (
          <Danger>{roleDiffProps.errorMessage}</Danger>
        )}

        {convertAttempt.status === 'success' && (
          <RoleEditor
            originalRole={convertAttempt.data}
            roleDiffAttempt={roleDiffProps?.roleDiffAttempt}
            onCancel={onCancel}
            onSave={onSave}
            onRoleUpdate={onRoleUpdate}
          />
        )}
      </Flex>
      <RoleEditorAdapterCompanion
        roleDiffProps={roleDiffProps}
        currentFlow={resources.status === 'creating' ? 'creating' : 'updating'}
      />
    </Flex>
  );
}

// RoleEditorAdapterCompanion is the component that will be shown
// next to the role editor adapter. This can either be the
// role diff visualizer, or the upsell
export function RoleEditorAdapterCompanion({
  roleDiffProps,
  currentFlow,
}: {
  roleDiffProps?: RoleDiffProps;
  currentFlow: 'creating' | 'updating';
}) {
  const ctx = useTeleport();
  const version = ctx.storeUser.state.cluster.authVersion;
  // the demo banner should show every time they load the role editor
  const [demoDismissed, setDemoDismissed] = useState(false);
  if (
    roleDiffProps &&
    (roleDiffProps.roleDiffState === RoleDiffState.DEMO_READY ||
      roleDiffProps.roleDiffState === RoleDiffState.POLICY_ENABLED)
  ) {
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
            roleDiffProps.roleDiffState === RoleDiffState.DEMO_READY && (
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
        roleDiffProps={roleDiffProps}
        enableDemoMode={roleDiffProps?.enableDemoMode}
        currentFlow={currentFlow}
      />
    </Flex>
  );
}
