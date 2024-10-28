import React from 'react';
import { Text, Link as ExternalLink, Flex } from 'design';
import { NewTab } from 'design/Icon';

import { OktaScimDetails } from 'teleport/services/integrations/oktaStatusTypes';
import cfg from 'teleport/config';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import { CtaEvent } from 'teleport/services/userEvent';

import { Panel, PanelTitle, CenteredFlex, CustomLabel } from './Shared';
import { generateOktaScimSettingsUrl } from './generateOktaAdminLink';

export function Scim({
  spec,
  orgUrl,
  appId,
  appName,
}: {
  spec: OktaScimDetails;
  orgUrl: string;
  appId: string;
  appName: string;
}) {
  const scimFeature = cfg.entitlements.OktaSCIM;
  const hasScim = scimFeature.enabled && scimFeature.limit === 0;
  return (
    <Panel>
      <CenteredFlex>
        <CenteredFlex>
          <PanelTitle>SCIM</PanelTitle>
        </CenteredFlex>
        <CustomLabel enabled={spec.enabled} />
      </CenteredFlex>
      <Text color="text.slightlyMuted">
        SCIM enables Okta to push user (and other resource) updates to Teleport
        in near real time, without waiting for the Okta integration to run a
        synchronization.
      </Text>
      {hasScim && (
        <>
          <Flex flexDirection="column" mt={3} mb={2}>
            <Text>SCIM Base URL:</Text>
            <TextSelectCopyMulti
              bash={false}
              lines={[
                {
                  text: `${cfg.baseUrl}/v1/webapi/scim/okta`,
                },
              ]}
            />
          </Flex>
          {appName && (
            <ExternalLink
              target="_blank"
              href={generateOktaScimSettingsUrl({
                orgUrl,
                appId,
                appName,
              })}
            >
              <Flex gap={1}>
                Open SCIM Settings in Okta
                <NewTab size={16} />
              </Flex>
            </ExternalLink>
          )}
        </>
      )}
      {!hasScim && (
        <ButtonLockedFeature mt={3} event={CtaEvent.CTA_OKTA_SCIM}>
          Unlock with Teleport Identity
        </ButtonLockedFeature>
      )}
    </Panel>
  );
}
