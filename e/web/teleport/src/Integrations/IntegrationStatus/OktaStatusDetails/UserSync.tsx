import { Link as InternalLink } from 'react-router-dom';

import { Flex, P3, Text } from 'design';
import { SyncAlt } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';
import { OktaUserSyncDetails } from 'teleport/services/integrations/oktaStatusTypes';
import { CtaEvent } from 'teleport/services/userEvent';

import { getDurationText } from './date';
import {
  CenteredFlex,
  CustomLabel,
  ErrorTooltip,
  LinkedInnerCard,
  Panel,
  PanelTitle,
} from './Shared';

export function UserSync({ spec }: { spec: OktaUserSyncDetails }) {
  const userSyncFeature = cfg.entitlements.OktaUserSync;
  const hasUserSync = userSyncFeature.enabled && userSyncFeature.limit === 0;

  return (
    <Panel>
      <CenteredFlex>
        <CenteredFlex>
          <PanelTitle>User Sync</PanelTitle>
        </CenteredFlex>
        <CustomLabel enabled={spec.enabled} />
      </CenteredFlex>
      {!hasUserSync && (
        <Flex
          flexDirection="column"
          justifyContent="space-between"
          height="100%"
        >
          <UserSyncExplanation />
          <ButtonLockedFeature mt={3} event={CtaEvent.CTA_OKTA_USER_SYNC}>
            Unlock with Teleport Identity
          </ButtonLockedFeature>
        </Flex>
      )}
      {hasUserSync && (
        <Flex
          flexDirection="column"
          justifyContent="space-between"
          height="100%"
        >
          <Flex gap={3}>
            <HoverTooltip tipContent="Go to Users">
              <LinkedInnerCard
                as={InternalLink}
                to={`${cfg.routes.users}?search=okta`}
              >
                <Text bold fontSize={6} mb={2}>
                  {spec.numUsers || 0}
                </Text>
                <Text bold>Users</Text>
                <UserSyncExplanation />
              </LinkedInnerCard>
            </HoverTooltip>
          </Flex>
          <Flex alignItems="center" mt={3}>
            <SyncAlt color="text.slightlyMuted" size="small" mr={1} />
            <P3 color="text.slightlyMuted">
              Last Synced: {getDurationText(spec.lastSuccess)}{' '}
              <ErrorTooltip
                statusCode={spec.statusCode}
                lastFailed={spec.lastFailed}
                error={spec.error}
              />
            </P3>
          </Flex>
        </Flex>
      )}
    </Panel>
  );
}

const UserSyncExplanation = () => (
  <Text color="text.slightlyMuted">
    Synchronization of Okta users with Teleport so the Teleport user list always
    includes your full Okta user list
  </Text>
);
