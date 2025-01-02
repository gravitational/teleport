import { Box, ButtonWarning, Link as ExternalLink, Flex, Text } from 'design';
import { NewTab, Trash } from 'design/Icon';

import { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';

import { AccessListsSync } from './AccessListSync';
import { AppGroupSync } from './AppGroupSync';
import { Scim } from './Scim';
import { FlexWrap } from './Shared';
import { SsoDetails } from './SsoDetails';
import { UserSync } from './UserSync';

export function OktaStatusDetails({
  plugin,
  deletePlugin,
}: {
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  deletePlugin(): void;
}) {
  const { spec } = plugin;
  const { details } = plugin.status;
  return (
    <Box>
      <Flex gap={1} mb={3} flexWrap="wrap">
        <Text color="text.slightlyMuted">Okta Organization:</Text>
        <ExternalLink target="_blank" href={spec.orgUrl}>
          <Flex gap={1}>
            {spec.orgUrl}
            <NewTab size={16} />
          </Flex>
        </ExternalLink>
      </Flex>
      <FlexWrap gap={3} mb={3}>
        {details.ssoDetails && (
          <SsoDetails
            spec={details.ssoDetails}
            orgUrl={spec.orgUrl}
            teleportSsoConnector={spec.teleportSsoConnector}
          />
        )}
        {details.scimDetails && (
          <Scim
            spec={details.scimDetails}
            orgUrl={spec.orgUrl}
            appId={details.ssoDetails.appId}
            appName={details.ssoDetails.appName}
          />
        )}
      </FlexWrap>
      <FlexWrap gap={3} mb={3}>
        {details.usersSyncDetails && (
          <UserSync spec={details.usersSyncDetails} />
        )}
        {details.accessListsSyncDetails && (
          <AccessListsSync
            spec={details.accessListsSyncDetails}
            defaultOwners={spec.defaultOwners}
          />
        )}
      </FlexWrap>
      <Flex gap={3} flexWrap="wrap" mb={3}>
        {details.appGroupSyncDetails && (
          <AppGroupSync spec={details.appGroupSyncDetails} />
        )}
      </Flex>
      <ButtonWarning size="large" onClick={deletePlugin} mt={2}>
        <Trash mr={2} />
        <Text>Delete Integration</Text>
      </ButtonWarning>
    </Box>
  );
}
