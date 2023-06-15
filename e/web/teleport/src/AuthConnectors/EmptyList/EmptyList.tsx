import React from 'react';
import styled from 'styled-components';
import { Box, Flex, Text } from 'design';
import { AuthProviderType } from 'shared/services';
import Card from 'design/Card';
import { State as ResourceState } from 'teleport/components/useResources';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';

import { CtaEvent } from 'teleport/services/userEvent';

import getSsoIcon from '../getSsoIcon';

export default function EmptyList({ onCreate, showLockedFeature }: Props) {
  return (
    <Card
      color="text.main"
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
      <Flex
        flexWrap="wrap"
        style={{ position: 'relative' }}
        justifyContent="center"
        mt="4"
        minWidth="224px"
      >
        {renderItem('github', onCreate, false, showLockedFeature)}
        {renderItem('oidc', onCreate, showLockedFeature, showLockedFeature)}
        {renderItem('saml', onCreate, showLockedFeature, showLockedFeature)}
        {showLockedFeature && (
          <LockedFeatureContainer>
            <ButtonLockedFeature event={CtaEvent.CTA_AUTH_CONNECTOR}>
              Unlock OIDC & SAML with Teleport Enterprise
            </ButtonLockedFeature>
          </LockedFeatureContainer>
        )}
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
  min-width: 224px;
  padding: 24px;
  margin: 16px 8px;
  backgroundColor: ${props.theme.colors.levels.surface};
  display: flex;
  flex-direction: column;
  transition: all 0.3s;
  border-radius: 4px;
  min-height: 190px;
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
  max-width: 450px;
  bottom: 0;
  right: 3rem;
`;

type Props = {
  onCreate: ResourceState['create'];
  showLockedFeature: boolean;
};
