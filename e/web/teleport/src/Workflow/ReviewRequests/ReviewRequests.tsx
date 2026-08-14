import { Link, useParams } from 'react-router';

import { Box, ButtonBorder, ButtonPrimary, Flex, Text } from 'design';
import { ArrowBack } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

import cfg from 'e-teleport/config';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';

import RequestList from './RequestList/RequestList';
import { RequestView } from './RequestView/RequestView';

const NewRequestButton = ({ clusterId }: { clusterId: string }) => {
  return (
    <ButtonPrimary
      as={Link}
      to={cfg.getNewAccessRequestRoute(clusterId)}
      textTransform="none"
      title="New Access Request"
      width="240px"
    >
      New Access Request
    </ButtonPrimary>
  );
};

const AccessAutomationButton = ({ disabled }: { disabled: boolean }) => {
  const accessAutomationButton = (
    <ButtonBorder
      as={Link}
      to={cfg.getAccessAutomationRoute()}
      textTransform="none"
      title="Set Up Access Automation"
      width="240px"
      disabled={disabled}
    >
      Set Up Access Automation
    </ButtonBorder>
  );

  if (disabled) {
    return (
      <HoverTooltip
        placement="bottom"
        tipContent={
          'You do not have access to read/list Access Monitoring Rules'
        }
      >
        {accessAutomationButton}
      </HoverTooltip>
    );
  }

  return accessAutomationButton;
};

export default function Workflow() {
  const { requestId } = useParams<{ requestId?: string }>();
  const { clusterId } = useStickyClusterId();

  const ctx = useTeleport();
  const amRuleAccess = ctx.storeUser.getAccessMonitoringRuleAccess();
  const hasReadRulesAccess = amRuleAccess.list && amRuleAccess.read;

  if (!requestId) {
    return (
      <FeatureBox>
        <FeatureHeader
          css={`
            border-bottom: none;
          `}
          gap={3}
        >
          <Box flex="1">
            <FeatureHeaderTitle>Access Requests</FeatureHeaderTitle>
          </Box>
          <AccessAutomationButton disabled={!hasReadRulesAccess} />
          <NewRequestButton clusterId={clusterId} />
        </FeatureHeader>
        <RequestList />
      </FeatureBox>
    );
  }

  return (
    <FeatureBox>
      <FeatureHeader>
        <FeatureHeaderTitle>
          <Flex alignItems="center">
            <ArrowBack
              as={Link}
              mr={2}
              size="large"
              color="text.main"
              to={cfg.getAccessRequestRoute()}
            />
            <Flex mr={4} alignItems="baseline">
              <Text mr={3}>Request</Text>
              <Text typography="body2">{requestId}</Text>
            </Flex>
          </Flex>
        </FeatureHeaderTitle>
      </FeatureHeader>
      <RequestView requestId={requestId} />
    </FeatureBox>
  );
}
