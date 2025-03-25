import { useHistory } from 'react-router-dom';

import { Flex, P3, Text } from 'design';
import { FeatureName } from 'design/constants';
import { Edit, SyncAlt, User } from 'design/Icon';

import {
  OktaIntegrationLevel,
  oktaIntegrationLevels,
  UpsellBulletList,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import cfg from 'teleport/config';
import {
  OktaUserSyncDetails,
  PluginOktaSyncStatusCode,
} from 'teleport/services/integrations/oktaStatusTypes';
import { CtaEvent } from 'teleport/services/userEvent';

import {
  ErrorTooltip,
  getDurationText,
  Panel,
  PanelTitle,
  StatusAndOptions,
} from './Shared';

export function UserSyncDetails({
  spec,
  disabled,
  toggled,
  onToggle,
}: {
  spec?: OktaUserSyncDetails;
  disabled?: boolean;
  toggled?: boolean;
  onToggle: () => void;
}) {
  const history = useHistory();
  const hasIdentity = cfg.entitlements.Identity.enabled;
  const hasSyncError = spec?.statusCode === PluginOktaSyncStatusCode.Error;
  const showContent = hasIdentity && (toggled || hasSyncError);

  return (
    <Panel>
      <Flex alignItems="center" justifyContent="space-between" gap={2}>
        <PanelTitle>User Sync</PanelTitle>
        <StatusAndOptions
          enabled={toggled}
          disabled={!hasIdentity}
          setEnabled={onToggle}
          options={[
            {
              label: 'Edit Configuration',
              onClick: () =>
                history.push(
                  cfg.getIntegrationStatusRoute(
                    'okta',
                    'okta',
                    OktaIntegrationLevel.USER_SYNC
                  )
                ),
              Icon: Edit,
              disabled: disabled || !toggled,
            },
            {
              label: 'Go to Users',
              onClick: () => history.push(`${cfg.routes.users}?search=okta`),
              Icon: User,
            },
          ]}
        />
      </Flex>
      <Flex
        flexDirection="column"
        justifyContent="space-between"
        height="100%"
        px={2}
        pt={1}
      >
        {showContent ? (
          <>
            <Flex flexDirection="column" gap={2}>
              <Flex flexDirection="column" gap={3}>
                <Text fontWeight={400} fontSize={10} css={{ lineHeight: 1 }}>
                  {spec?.numUsers || '-'}
                </Text>
                <Text bold>Users</Text>
              </Flex>
              <Text color="text.slightlyMuted">
                Okta users are imported and synced with Teleport so Teleport
                always includes all of your Okta users.
              </Text>
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
          </>
        ) : (
          <>
            <UpsellBulletList
              bullets={
                oktaIntegrationLevels[OktaIntegrationLevel.USER_SYNC].bullets
              }
            />
            {!hasIdentity && (
              <ButtonLockedFeature event={CtaEvent.CTA_OKTA_USER_SYNC} mt={1}>
                Unlock with {FeatureName.IdentityGovernance}
              </ButtonLockedFeature>
            )}
          </>
        )}
      </Flex>
    </Panel>
  );
}
