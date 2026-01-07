import { useHistory } from 'react-router-dom';

import { ButtonBorder, Flex, H2, Text } from 'design';
import { CardTile } from 'design/CardTile/CardTile';
import { FeatureName } from 'design/constants';
import { NewTab } from 'design/Icon';

import cfg from 'e-teleport/config';
import { StatusAndOptions } from 'e-teleport/Integrations/IntegrationStatus/Shared';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { CtaEvent } from 'teleport/services/userEvent';

/**
 * AccessGraphSyncDetails is the status of the Access Graph
 * sync configured for the Entra ID plugin.
 */
export function AccessGraphSyncDetails({
  syncEnabled,
}: {
  syncEnabled: boolean;
}) {
  const history = useHistory();
  const policyEnabled = cfg.oss.isPolicyEnabled;
  const syncAndPolicyEnabled = syncEnabled && policyEnabled;

  return (
    <CardTile
      maxWidth="30%"
      css={`
        @media screen and (max-width: ${p => p.theme.breakpoints.medium}) {
          max-width: 100%;
        }
      `}
    >
      <Flex alignItems="center" justifyContent="space-between" gap={2}>
        <H2>Access Graph Sync</H2>
        <StatusAndOptions enabled={syncAndPolicyEnabled} />
      </Flex>
      <Flex flexDirection="column" gap={3} px={1} pt={1}>
        <Text color="text.slightlyMuted" mb={3}>
          Analyze access paths with Teleport Access Graph.
        </Text>
        {syncAndPolicyEnabled ? (
          <ButtonBorder
            onClick={() => history.push(cfg.routes.accessGraph.dashboard)}
          >
            Open in Access Graph
            <NewTab ml={2} size="medium" />
          </ButtonBorder>
        ) : (
          <DisabledState
            syncEnabled={syncEnabled}
            policyEnabled={policyEnabled}
          />
        )}
      </Flex>
    </CardTile>
  );
}

function DisabledState({
  syncEnabled,
  policyEnabled,
}: {
  syncEnabled: boolean;
  policyEnabled: boolean;
}) {
  if (!syncEnabled && policyEnabled) {
    return (
      <Text color="text.slightlyMuted">
        Re-install the plugin to configure Access Graph sync.
      </Text>
    );
  }

  if (syncEnabled && !policyEnabled) {
    return (
      <>
        <Text color="text.slightlyMuted">
          Access Graph sync is configured but the feature is locked due to
          missing license.
        </Text>
        <UnlockPolicy />
      </>
    );
  }

  return <UnlockPolicy />;
}

function UnlockPolicy() {
  return (
    <ButtonLockedFeature event={CtaEvent.CTA_IDENTITY_SECURITY} mt={1}>
      Unlock with {FeatureName.IdentitySecurity}
    </ButtonLockedFeature>
  );
}
