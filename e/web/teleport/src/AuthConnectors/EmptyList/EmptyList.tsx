import React from 'react';
import styled from 'styled-components';
import { Text, Box, Flex } from 'design';
import { AuthProviderType } from 'shared/services';
import Card from 'design/Card';
import { State as ResourceState } from 'teleport/components/useResources';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

import getSsoIcon from '../getSsoIcon';

export default function EmptyList({ onCreate, showLockedFeature }: Props) {
  return (
    <Card
      color="text.main"
      bg="levels.surface"
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
      <Flex mt="6" flexWrap="wrap" justifyContent="center" minWidth="800px">
        {renderItem('github', onCreate, false, showLockedFeature)}
        <Flex
          flexWrap="wrap"
          style={{ position: 'relative' }}
          justifyContent="center"
        >
          {renderItem('oidc', onCreate, showLockedFeature, showLockedFeature)}
          {renderItem('saml', onCreate, showLockedFeature, showLockedFeature)}
          {showLockedFeature && (
            <LockedFeatureContainer>
              <ButtonLockedFeature>
                Unlock OIDC & SAML with Teleport Enterprise
              </ButtonLockedFeature>
            </LockedFeatureContainer>
          )}
        </Flex>
      </Flex>
    </Card>
  );
}

function renderItem(
  kind: AuthProviderType,
  onClick: Props['onCreate'],
  isItemLocked: boolean, // wether this particular item is locked
  isFeatureLocked: boolean // wether the enterprise auth connectors feature is locked
) {
  const { desc, SsoIcon, info } = getSsoIcon(kind, isFeatureLocked);
  const onBtnClick = () => onClick(kind);
  return (
    <ConnectorBox
      p="4"
      mx="2"
      mb="3"
      bg="levels.surface"
      as="button"
      disabled={isItemLocked}
      onClick={isItemLocked ? null : onBtnClick}
    >
      <Flex width="100%">
        <SsoIcon
          fontSize="50px"
          style={{
            left: 0,
            fontSize: '72px',
          }}
        />
      </Flex>

      <Text typography="body2" mt="4" fontSize="18px" color="text.primary" bold>
        {desc}
      </Text>
      {info && (
        <Text mt="2" color="text.primary" transform="none">
          {info}
        </Text>
      )}
    </ConnectorBox>
  );
}

const ConnectorBox = styled(Box)(
  props => `
  display: flex;
  flex-direction: column;
  transition: all 0.3s;
  border-radius: 4px;
  min-width: 340px;
  min-height: 190px;
  width: 160px;
  border: 2px solid ${props.theme.colors.spotBackground[2]};
  &:focus {
    opacity: .24;
    box-shadow: none;
  }
  &:hover:not([disabled]) {
    border: 2px solid ${props.theme.colors.brand.main};
  }
  &:hover {
    border: 2px solid ${props.theme.colors.brand};
    background: ${props.theme.colors.levels.elevated};
    box-shadow: 0 4px 14px rgba(0, 0, 0, 0.56);
    cursor: pointer;
  }
  color: inherit;
  font-family: inherit;
  outline: none;
  position: relative;
  text-align: center;
  text-decoration: none;
  &:disabled {
    opacity: .24;
    box-shadow: none;
  }
`
);

const LockedFeatureContainer = styled(Box)`
  position: absolute;
  min-width: 360px;
  bottom: 0;
  left: 3rem;
  right: 3rem;
`;

type Props = {
  onCreate: ResourceState['create'];
  showLockedFeature: boolean;
};
