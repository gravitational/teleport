/**
 * Copyright 2020 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
import styled from 'styled-components';
import { Text, Flex } from 'design';
import { MenuIcon, MenuItem } from 'shared/components/MenuAction';
import { App } from 'teleport/services/apps';
import { NewTab } from 'design/Icon';

export default function AppList({ apps = [] }: Props) {
  const $apps = apps.map(app => <Item mb={4} mr={5} app={app} key={app.id} />);
  return <Flex flexWrap="wrap">{$apps}</Flex>;
}

function Item(props: ItemProps) {
  const { app, ...rest } = props;
  return (
    <StyledAppListItem
      width="240px"
      height="240px"
      borderRadius="3"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      bg="primary.light"
      {...rest}
    >
      <Flex width="100%" justifyContent="center">
        <MenuIcon buttonIconProps={menuActionProps}>
          <MenuItem as="a" href={app.launchUrl} target="_blank">
            Open
          </MenuItem>
        </MenuIcon>
      </Flex>
      <Flex
        flex="1"
        alignItems="center"
        justifyContent="center"
        flexDirection="column"
        as="a"
        tabIndex={-1}
        target="_blank"
        color="text.primary"
        href={app.launchUrl}
        width="220px"
        px="2"
        style={{
          textDecoration: 'none',
        }}
      >
        <NewTab fontSize="62px" mb="3" />
        <Text style={textStyle} bold mb="2">
          {app.name}
        </Text>
      </Flex>
    </StyledAppListItem>
  );
}

const textStyle = {
  textAlign: 'center',
  width: '100%',
};

const StyledAppListItem = styled(Flex)`
  position: relative;
  box-shadow: 0 4px 32px rgba(0, 0, 0, 0.24);
  cursor: pointer;
  transition: 0.3s;
  border: 2px solid transparent;
  &:hover {
    border: 2px solid ${props => props.theme.colors.secondary.main};
    background: ${props => props.theme.colors.primary.lighter};
  }
`;

type Props = {
  apps: App[];
};

type ItemProps = {
  app: App;
  [name: string]: any;
};

const menuActionProps = {
  style: {
    right: '10px',
    position: 'absolute',
    top: '10px',
  },
};
