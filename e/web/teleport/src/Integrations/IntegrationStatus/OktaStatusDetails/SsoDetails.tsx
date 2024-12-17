import styled from 'styled-components';
import { Text, Link as ExternalLink, Flex, Box, Mark } from 'design';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { NewTab } from 'design/Icon';
import { Link as InternalLink } from 'react-router-dom';

import { OktaSsoDetails } from 'teleport/services/integrations/oktaStatusTypes';
import cfg from 'teleport/config';

import useTeleportE from 'e-teleport/useTeleportE';

import {
  Panel,
  PanelTitle,
  CenteredFlex,
  CustomLabel,
  LinkedInnerCard,
} from './Shared';
import { generateOktaSamlAppUrl } from './generateOktaAdminLink';

export function SsoDetails({
  spec,
  orgUrl,
  teleportSsoConnector,
}: {
  spec: OktaSsoDetails;
  orgUrl: string;
  teleportSsoConnector: string;
}) {
  const ctx = useTeleportE();
  const hasSsoAccess = ctx.storeUser.getConnectorAccess().list;
  return (
    <Panel>
      <CenteredFlex>
        <CenteredFlex>
          <PanelTitle>Single Sign-On</PanelTitle>
          <ToolTipInfo>
            A SAML SSO connector that grants Okta users the default role of
            requester
          </ToolTipInfo>
        </CenteredFlex>
        <CustomLabel enabled={spec.enabled} />
      </CenteredFlex>
      <Flex gap={2} height="100%">
        {hasSsoAccess && (
          <VerticallyCenteredFlex as={InternalLink} to={cfg.routes.sso}>
            {spec.appName ? (
              <>
                <Text>View</Text>
                <Text>Teleport's Connector</Text>
                <Text>
                  <Mark>{teleportSsoConnector}</Mark>
                </Text>
              </>
            ) : (
              <Text>
                Click to view Teleport's Connector named{' '}
                <Mark>{teleportSsoConnector}</Mark>
              </Text>
            )}
          </VerticallyCenteredFlex>
        )}
        {spec.appName && (
          <VerticallyCenteredFlex
            as={ExternalLink}
            href={generateOktaSamlAppUrl({
              orgUrl,
              appId: spec.appId,
              appName: spec.appName,
            })}
            target="_blank"
          >
            <Flex justifyContent="space-between" alignItems="center">
              <Box>
                <Text>Open</Text>
                <Text>Okta's SAML App</Text>
              </Box>
              <NewTab />
            </Flex>
          </VerticallyCenteredFlex>
        )}
      </Flex>
    </Panel>
  );
}

const VerticallyCenteredFlex = styled(LinkedInnerCard)`
  display: flex;
  flex-direction: column;
  justify-content: center;
  height: 100%;
  font-weight: normal;
`;
