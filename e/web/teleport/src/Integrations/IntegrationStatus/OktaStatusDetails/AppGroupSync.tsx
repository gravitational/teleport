import { Link as InternalLink } from 'react-router-dom';

import { Flex, P3, Text } from 'design';
import { SyncAlt } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'teleport/config';
import { OktaAppGroupSyncDetails } from 'teleport/services/integrations/oktaStatusTypes';

import { getDurationText } from './date';
import {
  CenteredFlex,
  CustomLabel,
  ErrorTooltip,
  FlexWrap,
  InnerCard,
  LinkedInnerCard,
  Panel,
  PanelTitle,
} from './Shared';

export function AppGroupSync({ spec }: { spec: OktaAppGroupSyncDetails }) {
  return (
    <Panel width="100%">
      <CenteredFlex>
        <CenteredFlex>
          <PanelTitle>Apps & User Groups Sync</PanelTitle>
        </CenteredFlex>
        <CustomLabel enabled={true} />
      </CenteredFlex>
      <FlexWrap gap={3}>
        <InnerCard>
          <Text bold fontSize={6} mb={2}>
            {spec.numGroups || 0}
          </Text>
          <Text bold>User Groups</Text>
          <Text color="text.slightlyMuted">
            Synchronization of Okta user groups imported as Teleport user groups
          </Text>
        </InnerCard>
        <HoverTooltip
          tipContent="Go to Applications"
          flexBasisProps={{ flexBasis: '100%' }}
        >
          <LinkedInnerCard
            as={InternalLink}
            to={`${cfg.getUnifiedResourcesRoute(cfg.proxyCluster)}?sort=name%3Aasc&kinds=app&query=labels%5B"teleport.dev%2Forigin"%5D+%3D%3D+"okta"`}
          >
            <Text bold fontSize={6} mb={2}>
              {spec.numApps || 0}
            </Text>
            <Text bold>Applications</Text>
            <Text color="text.slightlyMuted">
              Synchronization of Okta applications imported as Teleport
              application
            </Text>
          </LinkedInnerCard>
        </HoverTooltip>
      </FlexWrap>
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
    </Panel>
  );
}
