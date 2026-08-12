import { useNavigate } from 'react-router';

import { Flex, Label, P3, Text } from 'design';
import { FeatureName } from 'design/constants';
import { Application, Edit, SyncAlt, UserList } from 'design/Icon';

import cfg from 'e-teleport/config';
import {
  APP_GROUP_SYNC_CONFIG,
  OktaIntegrationStepType,
  UpsellBulletList,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import {
  ErrorTooltip,
  FlexWrap,
  getDurationText,
  Panel,
  PanelTitle,
} from 'e-teleport/Integrations/IntegrationStatus/OktaStatusDetails/Shared';
import { StatusAndOptions } from 'e-teleport/Integrations/IntegrationStatus/Shared';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import {
  OktaAccessListSyncDetails,
  OktaAppGroupSyncDetails,
  PluginOktaSyncStatusCode,
} from 'teleport/services/integrations/oktaStatusTypes';
import { CtaEvent } from 'teleport/services/userEvent';

export function AppGroupSyncDetails({
  appGroupSpec,
  accessListSpec,
  defaultOwners,
  toggled,
  disabled,
  onToggle,
}: {
  appGroupSpec?: OktaAppGroupSyncDetails;
  accessListSpec?: OktaAccessListSyncDetails;
  defaultOwners?: string[];
  toggled?: boolean;
  disabled?: boolean;
  onToggle: () => void;
}) {
  const navigate = useNavigate();
  const hasIdentity = cfg.oss.entitlements.Identity.enabled;
  const hasSyncError =
    appGroupSpec?.statusCode === PluginOktaSyncStatusCode.Error ||
    accessListSpec?.statusCode === PluginOktaSyncStatusCode.Error;
  const showContent = hasIdentity && (toggled || hasSyncError);

  return (
    <Panel width="100%">
      <Flex alignItems="center" justifyContent="space-between" gap={2}>
        <PanelTitle>Apps and User Groups</PanelTitle>
        <StatusAndOptions
          enabled={toggled}
          disabled={!hasIdentity}
          setEnabled={onToggle}
          options={[
            {
              label: 'Edit Configuration',
              onClick: () =>
                navigate(
                  cfg.oss.getIntegrationStatusRoute(
                    'okta',
                    'okta',
                    OktaIntegrationStepType.AppGroupSync
                  )
                ),
              Icon: Edit,
              disabled: disabled || !toggled,
            },
            {
              label: 'Go to Applications',
              onClick: () =>
                navigate(
                  `${cfg.oss.getUnifiedResourcesRoute(cfg.oss.proxyCluster)}?sort=name%3Aasc&kinds=app&query=labels%5B"teleport.dev%2Forigin"%5D+%3D%3D+"okta"`
                ),
              Icon: Application,
            },
            {
              label: 'Go to Access Lists',
              onClick: () =>
                navigate(`${cfg.getAccessListManagementRoute()}?search=okta`),
              Icon: UserList,
            },
          ]}
        />
      </Flex>
      {showContent ? (
        <>
          <FlexWrap gap={4} pt={2}>
            <FlexWrap gap={4} px={2} flex={1}>
              <AppGroupDetailsCard
                title="Apps"
                num={appGroupSpec?.numApps}
                desc="Synced as Teleport Application Resources"
              />
              <AppGroupDetailsCard
                title="User Groups"
                num={appGroupSpec?.numGroups}
                desc="Synced as Teleport User Group Resources"
              />
            </FlexWrap>
            <span
              css={`
                @media screen and (max-width: ${p =>
                  p.theme.breakpoints.tablet}) {
                  border-left: none;
                  border-top: 1px solid ${p => p.theme.colors.spotBackground[2]};
                  width: 100%;
                  height: 1px;
                }
                border-left: 1px solid ${p => p.theme.colors.spotBackground[2]};
                height: 100%;
                width: 1px;
              `}
            ></span>
            <Flex flexDirection="column" px={2} gap={3} flex={1}>
              <FlexWrap gap={4}>
                <AccessListDetailsCard
                  title="Access Lists from User Groups"
                  shortTitle="Groups"
                  num={accessListSpec?.numGroups}
                  desc="Used to grant long-term access, synced from Okta user groups with assigned apps"
                  filters={accessListSpec?.groupFilters}
                />
                <AccessListDetailsCard
                  title="Access Lists from Apps"
                  shortTitle="Apps"
                  num={accessListSpec?.numApps}
                  desc="Used to grant long-term access, synced from Okta apps with direct assignments"
                  filters={accessListSpec?.appFilters}
                />
              </FlexWrap>
              {defaultOwners?.length ? (
                <Flex gap={2} flexWrap="wrap" width="100%">
                  <Text>Default Owners:</Text>
                  {defaultOwners.map((label, index) => (
                    <Label key={`${label}${index}`} kind="secondary">
                      {label}
                    </Label>
                  ))}
                </Flex>
              ) : (
                <Text color={'text.slightlyMuted'}>
                  <i>No default owners.</i>
                </Text>
              )}
            </Flex>
          </FlexWrap>
          <Flex alignItems="center" mt={3} px={1}>
            <SyncAlt color="text.slightlyMuted" size="small" mr={1} />
            <P3 color="text.slightlyMuted">
              Last Synced:{' '}
              {getDurationText(
                appGroupSpec?.lastSuccess || accessListSpec?.lastSuccess
              )}{' '}
              <ErrorTooltip
                statusCode={
                  appGroupSpec?.statusCode || accessListSpec?.statusCode
                }
                lastFailed={
                  appGroupSpec?.lastFailed || accessListSpec?.lastFailed
                }
                error={appGroupSpec?.error || accessListSpec?.error}
              />
            </P3>
          </Flex>
        </>
      ) : (
        <Flex
          flexDirection="column"
          justifyContent="space-between"
          height="100%"
        >
          <UpsellBulletList bullets={APP_GROUP_SYNC_CONFIG.bullets} />
          {!hasIdentity && (
            <ButtonLockedFeature event={CtaEvent.CTA_OKTA_USER_SYNC} mt={3}>
              Unlock with {FeatureName.IdentityGovernance}
            </ButtonLockedFeature>
          )}
        </Flex>
      )}
    </Panel>
  );
}

const AppGroupDetailsCard = ({
  title,
  num,
  desc,
}: {
  title: string;
  num?: number;
  desc: string;
}) => (
  <Flex flexDirection="column" gap={2} flex={1}>
    <Flex flexDirection="column" gap={3}>
      <Text fontWeight={400} fontSize={10} css={{ lineHeight: 1 }}>
        {num || '-'}
      </Text>
      <Text bold>{title}</Text>
    </Flex>
    <Text color="text.slightlyMuted">{desc}</Text>
  </Flex>
);

const AccessListDetailsCard = ({
  title,
  shortTitle,
  num,
  desc,
  filters,
}: {
  title: string;
  shortTitle: string;
  num: number;
  desc: string;
  filters?: string[];
}) => (
  <Flex
    flexDirection="column"
    justifyContent="space-between"
    alignItems="flex-start"
    gap={2}
    flex={1}
  >
    <Flex flexDirection="column" gap={2} width="100%">
      <Flex flexDirection="column" gap={3}>
        <Text fontWeight={400} fontSize={10} css={{ lineHeight: 1 }}>
          {num || '-'}
        </Text>
        <Text bold>{title}</Text>
      </Flex>
      <Text color="text.slightlyMuted">{desc}</Text>
    </Flex>
    <Flex
      flexDirection="column"
      gap={2}
      pt={2}
      width="100%"
      css={`
        border-top: 1px solid ${p => p.theme.colors.spotBackground[0]};
      `}
    >
      {filters?.length ? (
        <Flex gap={1} flexWrap="wrap" width="100%">
          <Text>{shortTitle} Filtered By:</Text>
          {filters.map((label, index) => (
            <Label key={`${label}${index}`} kind="secondary">
              {label}
            </Label>
          ))}
        </Flex>
      ) : (
        <Text color={'text.slightlyMuted'}>
          <i>
            All {num || 0} {shortTitle} are being synced.
          </i>
        </Text>
      )}
    </Flex>
  </Flex>
);
