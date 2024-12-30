/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import styled, { useTheme } from 'styled-components';

import { Flex, H2, Subtitle2, ButtonSecondary, P3, Box } from 'design';
import { ArrowRight, CircleCheck, Password } from 'design/Icon';

import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import { AuthType } from 'shared/services';

import { State as ResourceState } from 'teleport/components/useResources';

export function AuthConnectorTile({
  id,
  name,
  kind,
  Icon,
  isDefault,
  isPlaceholder,
  onSetup,
  customDesc,
  onEdit,
  onDelete,
}: {
  name: string;
  id: string;
  kind: AuthType;
  Icon: () => JSX.Element;
  isDefault: boolean;
  /**
   * isPlaceholder is whether this isn't a real existing connector, but a placeholder as a shortcut to set one up.
   */
  isPlaceholder: boolean;
  onSetup?: () => void;
  customDesc?: string;
  onEdit?: ResourceState['edit'];
  onDelete?: ResourceState['remove'];
}) {
  const theme = useTheme();
  const onClickEdit = () => onEdit(id);
  const onClickDelete = () => onDelete(id);

  let desc: string;
  switch (kind) {
    case 'github':
      desc = 'GitHub Connector';
      break;
    case 'oidc':
      desc = 'OIDC Connector';
      break;
    case 'saml':
      desc = 'SAML Connector';
      break;
    default:
      kind satisfies never | 'local';
  }

  return (
    <ConnectorBox tabIndex={0}>
      <Flex
        flexDirection="column"
        justifyContent="space-between"
        alignItems="flex-start"
        height="100%"
        gap={3}
      >
        <Icon />
        <Flex flexDirection="column" alignItems="flex-start" gap={1}>
          <Flex alignItems="center" gap={2}>
            <H2>{name}</H2>
            {isDefault && <DefaultIndicator />}
          </Flex>
          <Subtitle2
            css={`
              display: -webkit-box;
              -webkit-box-orient: vertical;
              -webkit-line-clamp: 1;
            `}
            color="text.slightlyMuted"
          >
            {customDesc || desc}
          </Subtitle2>
        </Flex>
      </Flex>
      <Flex
        flexDirection="column"
        justifyContent="space-between"
        alignItems="flex-end"
        height="100%"
      >
        {isPlaceholder && !!onSetup && (
          <ButtonSecondary onClick={onSetup} px={3}>
            Set Up <ArrowRight size="small" ml={2} />
          </ButtonSecondary>
        )}
        {!isPlaceholder && !!onEdit && !!onDelete && (
          <MenuIcon
            buttonIconProps={{ style: { borderRadius: `${theme.radii[2]}px` } }}
          >
            <MenuItem onClick={onClickEdit}>Edit</MenuItem>
            <MenuItem onClick={onClickDelete}>Delete</MenuItem>
          </MenuIcon>
        )}
      </Flex>
    </ConnectorBox>
  );
}

/**
 * LocalConnectorTile is a hardcoded "auth connector" which represents local auth.
 */
export function LocalConnectorTile() {
  return (
    <AuthConnectorTile
      key="local-auth-tile"
      kind="local"
      id="local"
      Icon={() => (
        <Flex
          alignItems="center"
          justifyContent="center"
          css={`
            background-color: ${props =>
              props.theme.colors.interactive.tonal.neutral[0]};
            height: 61px;
            width: 61px;
          `}
          lineHeight={0}
          p={2}
          borderRadius={3}
        >
          <Password size="extra-large" />
        </Flex>
      )}
      isDefault={true}
      isPlaceholder={false}
      name="Local Connector"
      customDesc="Manual auth w/ users local to Teleport"
    />
  );
}

export const ConnectorBox = styled(Box)<{ disabled?: boolean }>`
  display: flex;
  flex-direction: row;
  justify-content: space-between;
  align-items: flex-start;
  font-family: ${props => props.theme.font};
  padding: ${p => p.theme.space[3]}px;
  transition: all 0.3s;
  border-radius: ${props => props.theme.radii[2]}px;
  border: ${props => props.theme.borders[2]}
    ${props => props.theme.colors.interactive.tonal.neutral[0]};

  &:hover {
    background: ${props => props.theme.colors.levels.surface};
    border: ${props => props.theme.borders[2]} transparent;
  }

  &:focus-visible {
    outline: none;
    border: ${props => props.theme.borders[2]}
      ${props => props.theme.colors.text.muted};
  }

  &:active {
    outline: none;
    background: ${props => props.theme.colors.levels.surface};
    border: ${props => props.theme.borders[2]}
      ${props => props.theme.colors.interactive.tonal.neutral[1]};
  }
`;

function DefaultIndicator() {
  return (
    <Flex
      justifyContent="center"
      alignItems="center"
      gap={1}
      py={1}
      px={2}
      css={`
        background-color: ${props =>
          props.theme.colors.interactive.tonal.success[1]};
        border-radius: 62px;
      `}
    >
      <CircleCheck color="interactive.solid.success.default" size="small" />
      <P3 color="interactive.solid.success.default" mr="2px">
        Default
      </P3>
    </Flex>
  );
}
