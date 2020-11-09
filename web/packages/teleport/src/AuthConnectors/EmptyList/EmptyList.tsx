/*
Copyright 2020 Gravitational, Inc.

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
import { Text, Box, Flex } from 'design';
import { AuthProviderType } from 'shared/services';
import Card from 'design/Card';
import getSsoIcon from '../getSsoIcon';

export default function EmptyList({ onCreate }: Props) {
  return (
    <Card
      color="text.primary"
      bg="primary.light"
      p="5"
      textAlign="center"
      style={{ boxShadow: 'none' }}
    >
      <Text typography="h3" textAlign="center">
        Create Your First Auth Connector
        <Text typography="subtitle1" mt="2">
          Select a service provider below to create your first Authentication
          Connector.
        </Text>
      </Text>
      <Flex mt="6" flexWrap="wrap">
        {renderItem('github', onCreate)}
        {renderItem('oidc', onCreate)}
        {renderItem('saml', onCreate)}
      </Flex>
    </Card>
  );
}

function renderItem(kind: AuthProviderType, onClick: Props['onCreate']) {
  const { desc, SsoIcon } = getSsoIcon(kind);
  const onBtnClick = () => onClick(kind);
  return (
    <ConnectorBox
      px="5"
      py="4"
      mx="2"
      mb="3"
      bg="primary.light"
      as="button"
      onClick={onBtnClick}
    >
      <SsoIcon fontSize="50px" my={2} />
      <Text typography="body2" bold>
        {desc}
      </Text>
    </ConnectorBox>
  );
}

const ConnectorBox = styled(Box)(
  props => `
  display: flex;
  align-items: center;
  flex-direction: column;
  transition: all 0.3s;
  border-radius: 4px;
  width: 160px;
  border: 2px solid ${props.theme.colors.primary.main};

  &:focus {
    opacity: .24;
    box-shadow: none;
  }

  &:hover {
    border: 2px solid ${props.theme.colors.secondary.main};
    background: ${props.theme.colors.primary.lighter};
    box-shadow: 0 4px 14px rgba(0, 0, 0, 0.56);
  }

  color: inherit;
  cursor: pointer;
  font-family: inherit;
  outline: none;
  position: relative;
  text-align: center;
  text-decoration: none;
  text-transform: uppercase;
`
);

type Props = {
  onCreate(kind: AuthProviderType): void;
};
