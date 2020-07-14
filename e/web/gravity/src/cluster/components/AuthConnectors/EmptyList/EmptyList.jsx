import React from 'react';
import styled from 'styled-components';
import { Text, Box, Flex } from 'design';
import Card from 'design/Card';
import getSsoIcon from './../getSsoIcon';

export default function EmptyList({ onCreate }) {
  return (
    <Card color="text.primary" bg="primary.light" p="6">
      <Text typography="h2" textAlign="center">
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

function renderItem(kind, onClick) {
  const { desc, SsoIcon } = getSsoIcon(kind);
  const onBtnClick = () => onClick(kind);
  return (
    <StyledConnectorBox
      px="5"
      py="4"
      mr="4"
      mb="4"
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

const StyledConnectorBox = styled(Box)`
  display: flex;
  align-items: center;
  flex-direction: column;
  transition: all 0.3s;
  border-radius: 4px;
  width: 160px;
  border: 2px solid ${({ theme }) => theme.colors.primary.main};
  &:hover {
    border: 2px solid ${({ theme }) => theme.colors.secondary.main};
  }

  &:focus {
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
`;
