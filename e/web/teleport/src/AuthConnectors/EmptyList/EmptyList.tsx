import React from 'react';
import { Flex, H1, Text } from 'design';
import { AuthProviderType } from 'shared/services';
import Card from 'design/Card';
import { State as ResourceState } from 'teleport/components/useResources';

import { CtaEvent } from 'teleport/services/userEvent';

import { ConnectorBox } from 'teleport/AuthConnectors/styles/ConnectorBox.styles';
import {
  LockedFeatureButton,
  LockedFeatureContainer,
} from 'teleport/AuthConnectors/styles/LockedFeatureContainer.styles';

import { H2 } from 'design';

import getSsoIconE from '../getSsoIconE';

export default function EmptyList({ onCreate, showLockedFeature }: Props) {
  return (
    <Card
      color="text.main"
      p={5}
      textAlign="center"
      style={{ boxShadow: 'none' }}
    >
      <H1 textAlign="center">Select a service provider below</H1>
      <Flex flexWrap="wrap" justifyContent="center" mt={4} minWidth="224px">
        {renderItem('github', onCreate, false, showLockedFeature)}
        <LockedFeatureContainer>
          {renderItem('oidc', onCreate, showLockedFeature, showLockedFeature)}
          {renderItem('saml', onCreate, showLockedFeature, showLockedFeature)}
          {showLockedFeature && (
            <LockedFeatureButton event={CtaEvent.CTA_AUTH_CONNECTOR}>
              Unlock OIDC & SAML with Teleport Enterprise
            </LockedFeatureButton>
          )}
        </LockedFeatureContainer>
      </Flex>
    </Card>
  );
}

function renderItem(
  kind: AuthProviderType,
  onClick: Props['onCreate'],
  isItemLocked: boolean, // whether this particular item is locked
  isFeatureLocked: boolean // whether the enterprise auth connectors feature is locked
) {
  const { desc, SsoIcon, info } = getSsoIconE(kind, isFeatureLocked);
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

      <H2 mt={4}>{desc}</H2>
      {info && (
        <Text mt={2} color="text.primary">
          {info}
        </Text>
      )}
    </ConnectorBox>
  );
}

type Props = {
  onCreate: ResourceState['create'];
  showLockedFeature: boolean;
};
