import { Link, useNavigate } from 'react-router';
import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import { ArrowBack } from 'design/Icon';
import { H1 } from 'design/Text';

import cfg from 'e-teleport/config';
import getSsoIcon from 'teleport/AuthConnectors/ssoIcons/getSsoIcon';
import { FeatureBox, FeatureHeaderTitle } from 'teleport/components/Layout';
import { KindAuthConnectors } from 'teleport/services/resources';

import { AddNewConnectorTile } from './AddNewConnectorTile';

export function AddNewConnectorPage() {
  return (
    <FeatureBox>
      <FeatureHeaderTitle py={3} mb={2}>
        <Flex alignItems="center">
          <ArrowBack
            as={Link}
            mr={2}
            size="large"
            color="text.main"
            to={cfg.oss.routes.sso}
          />
          <Flex mr={4} alignItems="baseline">
            <H1>Select an Auth Connector to set up</H1>
          </Flex>
        </Flex>
      </FeatureHeaderTitle>
      <AddNewConnectorsList />
    </FeatureBox>
  );
}

export function AddNewConnectorsList() {
  const navigate = useNavigate();

  const onCreate = (kind: KindAuthConnectors) => {
    navigate(cfg.oss.getCreateAuthConnectorRoute(kind));
  };

  return (
    <Box>
      <AddNewConnectorsGrid>
        <AddNewConnectorTile
          key="github"
          kind="github"
          name={'GitHub'}
          Icon={getSsoIcon('github')}
          onClick={() => onCreate('github')}
        />
        <AddNewConnectorTile
          key="okta"
          kind="saml"
          isGuided={true}
          name={'Okta'}
          Icon={getSsoIcon('saml', 'okta')}
          onClick={() => navigate(cfg.oss.getIntegrationEnrollRoute('okta'))}
        />
        {cfg.oss.entitlements.Identity.enabled && (
          <AddNewConnectorTile
            key="entra"
            kind="saml"
            isGuided={true}
            name={'Microsoft Entra ID'}
            Icon={getSsoIcon('saml', 'entraid')}
            onClick={() =>
              navigate(cfg.oss.getIntegrationEnrollRoute('entra-id'))
            }
          />
        )}
        <AddNewConnectorTile
          key="oidc"
          kind="oidc"
          name={'OIDC Connector'}
          customDesc="Google, GitLab, Amazon and more"
          Icon={getSsoIcon('oidc')}
          onClick={() => onCreate('oidc')}
        />
        <AddNewConnectorTile
          key="saml"
          kind="saml"
          name={'SAML Connector'}
          customDesc="OneLogin, Auth0, etc."
          Icon={getSsoIcon('saml')}
          onClick={() => onCreate('saml')}
        />
      </AddNewConnectorsGrid>
    </Box>
  );
}

export const AddNewConnectorsGrid = styled(Box)`
  width: 100%;
  display: grid;
  gap: ${p => p.theme.space[3]}px;
  grid-template-columns: repeat(auto-fill, minmax(336px, 1fr));
`;
