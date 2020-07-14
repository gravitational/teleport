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
    <StyledConnectorBox
      px="5"
      py="4"
      mx="2"
      mb="0"
      bg="primary.light"
      as="button"
      onClick={onBtnClick}
    >
      <SsoIcon fontSize="50px" my={2} />
      <Text typography="body2" bold>
        {desc}
      </Text>
    </StyledConnectorBox>
  );
}

const StyledConnectorBox = styled(Box)(
  props => `
  display: flex;
  align-items: center;
  flex-direction: column;
  transition: all 0.3s;
  border-radius: 4px;
  width: 160px;
  margin-bottom: 16px;
  border: 2px solid ${props.theme.colors.primary.main};
  &:hover {
    border: 2px solid ${props.theme.colors.secondary.main};
  }

  &:focus {
    opacity: .24;
    box-shadow: none;
  }

  &:hover {
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
