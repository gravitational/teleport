import React, { useState } from 'react';
import { Link } from 'react-router-dom';
import { useParams } from 'react-router';
import { Transition } from 'react-transition-group';
import { Text, Flex, ButtonPrimary, Box, ButtonText } from 'design';
import { ArrowBack, NewTab } from 'design/Icon';
import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import useStickyClusterId from 'teleport/useStickyClusterId';
import useTeleport from 'teleport/useTeleport';
import { HoverTooltip } from 'shared/components/ToolTip';

import cfg from 'e-teleport/config';

import RequestList from './RequestList/RequestList';
import { RequestView } from './RequestView/RequestView';
import { NotificationRoutingRulesDialog } from './NotificationRoutingRules/NotificationRoutingRulesDialog';

const NewRequestButton = ({ clusterId }: { clusterId: string }) => {
  return (
    <Link
      to={{
        pathname: `${cfg.getNewAccessRequestRoute(clusterId)}`,
      }}
      style={{ textDecoration: 'none', marginLeft: 'auto' }}
    >
      <ButtonPrimary
        textTransform="none"
        title="New Access Request"
        width="240px"
      >
        New Access Request
      </ButtonPrimary>
    </Link>
  );
};

export default function Workflow() {
  const [showRoutingRuleDialog, setShowRoutingRuleDialog] = useState(false);
  const { requestId } = useParams<{ requestId?: string }>();
  const { clusterId } = useStickyClusterId();

  const ctx = useTeleport();
  const amRuleAccess = ctx.storeUser.getAccessMonitoringRuleAccess();
  const hasReadRulesAccess = amRuleAccess.list && amRuleAccess.read;

  const ViewRulesButton = (
    <ButtonText
      onClick={() => setShowRoutingRuleDialog(true)}
      pl={0}
      css={{ fontWeight: 'normal' }}
      disabled={!hasReadRulesAccess}
    >
      <Flex alignItems="center" gap={2}>
        <Text>View Notification Routing Rules</Text>
        <NewTab size={18} />
      </Flex>
    </ButtonText>
  );

  if (!requestId) {
    return (
      <>
        <FeatureBox px={4}>
          <Flex alignItems="center" mb={4}>
            <Box mb={1}>
              <FeatureHeader
                alignItems="center"
                justifyContent="space-between"
                css={`
                  border-bottom: none;
                `}
                mb={-3}
              >
                <FeatureHeaderTitle>Access Requests</FeatureHeaderTitle>
              </FeatureHeader>
              {hasReadRulesAccess ? (
                <>{ViewRulesButton}</>
              ) : (
                <HoverTooltip
                  tipContent={
                    'You do not have access to read/list Notification Routing Rules'
                  }
                >
                  {ViewRulesButton}
                </HoverTooltip>
              )}
            </Box>
            <NewRequestButton clusterId={clusterId} />
          </Flex>
          <RequestList />
        </FeatureBox>
        <Transition
          in={showRoutingRuleDialog}
          timeout={300}
          mountOnEnter
          unmountOnExit
        >
          {transitionState => (
            <NotificationRoutingRulesDialog
              onClose={() => setShowRoutingRuleDialog(false)}
              transitionState={transitionState}
            />
          )}
        </Transition>
      </>
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
              <Text typography="body1">{requestId}</Text>
            </Flex>
          </Flex>
        </FeatureHeaderTitle>
      </FeatureHeader>
      <RequestView requestId={requestId} />
    </FeatureBox>
  );
}
