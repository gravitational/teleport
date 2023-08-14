/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { Close as CloseIcon } from 'design/Icon';
import { space } from 'design/system';
import { Flex, Text } from 'design';

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

const StyledTabItem = styled(Flex)`
  max-width: 200px;
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

  &:hover {
    color: ${props => props.theme.colors.text.primaryInverse};
    background: ${props => props.theme.colors.error.main};
  }

  ${space}
`;
