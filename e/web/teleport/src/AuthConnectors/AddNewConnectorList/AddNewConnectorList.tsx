import { useHistory } from 'react-router';
import styled from 'styled-components';

import Box from 'design/Box';
import { H2 } from 'design/Text';

import cfg from 'e-teleport/config';
import getSsoIcon from 'teleport/AuthConnectors/ssoIcons/getSsoIcon';
import { State as ResourceState } from 'teleport/components/useResources';

import { AddNewConnectorTile } from './AddNewConnectorTile';

export default function AddNewConnectorsList({
  onCreate,
}: {
  onCreate: ResourceState['create'];
}) {
  const history = useHistory();

  return (
    <Box>
      <H2 mb={4}>Enroll a Single Sign-On Connector</H2>
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
          onClick={() =>
            history.push(cfg.oss.getIntegrationEnrollRoute('okta'))
          }
        />
        {cfg.oss.entitlements.Identity.enabled && (
          <AddNewConnectorTile
            key="entra"
            kind="saml"
            isGuided={true}
            name={'Microsoft Entra ID'}
            Icon={getSsoIcon('saml', 'entraid')}
            onClick={() =>
              history.push(cfg.oss.getIntegrationEnrollRoute('entra-id'))
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
