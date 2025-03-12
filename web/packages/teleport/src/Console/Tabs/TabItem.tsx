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

import styled from 'styled-components';

import { Flex, Text } from 'design';
import { Cross as CloseIcon } from 'design/Icon';
import { space } from 'design/system';

import JoinedUsers from './JoinedUsers';

export default function TabItem(props: Props) {
  const { name, users, active, onClick, onClose, style } = props;
  return (
    <StyledTabItem alignItems="center" active={active} style={style}>
      <StyledTabButton onClick={onClick}>
        <JoinedUsers mr="1" users={users} active={active} />
        <Text mx="auto" title={name}>
          {name}
        </Text>
      </StyledTabButton>
      <StyledCloseButton title="Close" onClick={onClose}>
        <CloseIcon
          size="small"
          css={`
            transition: none;
            color: inherit;
          `}
        />
      </StyledCloseButton>
    </StyledTabItem>
  );
}

type Props = {
  name: string;
  users: { user: string }[];
  active: boolean;
  onClick: () => void;
  onClose: () => void;
  style: any;
};

function fromProps({ theme, active }) {
  let styles: Record<any, any> = {
    border: 'none',
    borderRight: `1px solid ${theme.colors.levels.sunken}`,
    '&:hover, &:focus': {
      color: theme.colors.text.main,
      transition: 'color .3s',
    },
  };

  if (active) {
    styles = {
      ...styles,
      backgroundColor: theme.colors.levels.sunken,
      color: theme.colors.text.main,
      fontWeight: 'bold',
      transition: 'none',
    };
  }

  return styles;
}

const StyledTabItem = styled(Flex)<{ active?: boolean }>`
  max-width: 450px;
  height: 100%;
  ${fromProps}
`;

const StyledTabButton = styled.button`
  display: flex;
  flex: 1;
  align-items: center;
  cursor: pointer;
  text-decoration: none;
  outline: none;
  margin: 0;
  color: inherit;
  line-height: 32px;
  background-color: transparent;
  white-space: nowrap;
  overflow: hidden;
  padding: 0 16px;
  text-overflow: ellipsis;
  border: none;
`;

const StyledCloseButton = styled.button`
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border-radius: 2px;
  border: none;
  cursor: pointer;
  height: 16px;
  width: 16px;
  outline: none;
  padding: 0;
  margin: 0 8px 0 0;
  transition: all 0.3s;
  color: ${props => props.theme.colors.text.main};

  &:hover {
    color: ${props => props.theme.colors.text.primaryInverse};
    background: ${props => props.theme.colors.error.main};
  }

  ${space}
`;
