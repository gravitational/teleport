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
import styled, { useTheme } from 'styled-components';

import { Danger } from 'design/Alert';
import Flex from 'design/Flex';
import { Indicator } from 'design/Indicator';
import { useAsync } from 'shared/hooks/useAsync';
import { debounce } from 'shared/utils/highbar';

import { State as ResourcesState } from 'teleport/components/useResources';
import { Role, RoleWithYaml } from 'teleport/services/resources';
import { yamlService } from 'teleport/services/yaml';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';

import { RoleDiffState, RolesProps } from '../Roles';
import { RoleEditor } from './RoleEditor';
import {
  RoleEditorVisualizer,
  shouldShowRoleDiff,
} from './RoleEditorVisualizer';

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
  const showRoleDiff = roleDiffProps && shouldShowRoleDiff(roleDiffProps);
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

  const [editorMinimized, setEditorMinimized] = useState(false);

  const originalContent = resources.item?.content ?? '';
  useEffect(() => {
    convertToRole(originalContent);
  }, [originalContent]);

  const onRoleUpdate = useCallback(
    debounce(role => roleDiffProps?.updateRoleDiff(role), 500),
    []
  );

  return (
    <Container flex="1">
      {/* This component's width influences how we lay out the permission
          checkboxes in AdminRules. */}
      <EditorPane
        minimized={editorMinimized}
        flexDirection="column"
        borderLeft={1}
        borderColor={theme.colors.interactive.tonal.neutral[0]}
        backgroundColor={theme.colors.levels.surface}
        width="35%"
        minWidth="494px"
        maxWidth="672px"
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

        {convertAttempt.status === 'success' && (
          <RoleEditor
            originalRole={convertAttempt.data}
            roleDiffAttempt={roleDiffProps?.roleDiffAttempt}
            onCancel={onCancel}
            onSave={onSave}
            onRoleUpdate={onRoleUpdate}
            demoMode={roleDiffProps?.roleDiffState === RoleDiffState.DemoReady}
            minimized={editorMinimized}
            onMinimizedChange={showRoleDiff ? setEditorMinimized : undefined}
          />
        )}
      </EditorPane>
      <RoleEditorVisualizer
        roleDiffProps={roleDiffProps}
        currentFlow={resources.status === 'creating' ? 'creating' : 'updating'}
      />
    </Container>
  );
}

const Container = styled(Flex)`
  position: relative;
`;

const EditorPane = styled(Flex)<{ minimized: boolean }>`
  position: ${props => (props.minimized ? 'absolute' : 'static')};
  left: ${props => props.theme.space[4]}px;
  top: ${props => props.theme.space[4]}px;

  border-top-left-radius: ${props =>
    props.minimized ? props.theme.radii[3] : 0}px;
  border-top-right-radius: ${props => props.theme.radii[3]}px;
  border-bottom-right-radius: ${props => props.theme.radii[3]}px;
  border-bottom-left-radius: ${props =>
    props.minimized ? props.theme.radii[3] : 0}px;
  box-shadow: ${props => props.theme.boxShadow[3]};

  // The editor pane needs to appear on top, even though it's before the
  // visualizer in the DOM tree.
  z-index: 1;
`;
