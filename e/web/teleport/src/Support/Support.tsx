import styled from 'styled-components';

import { Flex, Text } from 'design';
import { Clock, Edit } from 'design/Icon';
import { H2 } from 'design/Text';

import useTeleportE from 'e-teleport/useTeleportE';
import { ExternalAuditStorageCta } from 'teleport/components/ExternalAuditStorageCta';
import cfg from 'teleport/config';
import { Support } from 'teleport/Support';
import {
  DataItem,
  IconBox,
  SupportSectionCard,
} from 'teleport/Support/Support';

import { ClientIpRestrictions } from './ClientIpRestrictions';
import { Contacts } from './Contacts';
import { makeLabel, ScheduleUpgrades } from './ScheduleUpgrades';
import { useUpgradeWindowStart } from './useUpgradeWindowStart';

export default function Container() {
  return (
    <Support>
      <SupportE />
    </Support>
  );
}

export const SupportE = () => {
  const ctx = useTeleportE();
  const {
    scheduleUpgradesVisible,
    onUpdate,
    closeScheduleUpgrade,
    selectedUpgradeWindowStart,
    showScheduleUpgrade,
    setSelectedUpgradeWindowStart,
    updateWindowAttempt,
  } = useUpgradeWindowStart(ctx, ctx.storeUser.getClusterId(), cfg.isCloud);
  return (
    <>
      {cfg.isCloud && (
        <SupportSectionCard>
          <Flex alignItems="center" justifyContent="start" mb={3}>
            <IconBox>
              <Clock size={16} />
            </IconBox>
            <H2>Scheduled Upgrades</H2>
          </Flex>
          <DataItem
            title="Window Start Time"
            data={
              <Flex alignItems="center">
                {makeLabel(selectedUpgradeWindowStart)}
                <EditLink onClick={showScheduleUpgrade} ml="2" size="medium" />
              </Flex>
            }
          />
          <Text typography="body2" ml={{ _: 2, small: 0 }}>
            Window Start Time is the hour in which an upgrade may begin.
            Changing this value changes it for everyone in your organization.
          </Text>
        </SupportSectionCard>
      )}
      {(cfg.isCloud || cfg.isDashboard) && (
        <Contacts clusterId={ctx.storeUser.state.cluster.clusterId} />
      )}
      {cfg.isCloud && cfg.entitlements.ClientIPRestrictions.enabled && (
        <ClientIpRestrictions
          clusterId={ctx.storeUser.state.cluster.clusterId}
        />
      )}
      <ExternalAuditStorageCta />
      {scheduleUpgradesVisible && (
        <ScheduleUpgrades
          onSave={onUpdate}
          onCancel={closeScheduleUpgrade}
          selectedWindow={selectedUpgradeWindowStart}
          onSelectedWindowChange={setSelectedUpgradeWindowStart}
          attempt={updateWindowAttempt}
        />
      )}
    </>
  );
};

const EditLink = styled(Edit)`
  color: ${props => props.theme.colors.text.slightlyMuted};
  &:hover,
  &:focus {
    color: ${props => props.theme.colors.text.main};
    cursor: pointer;
  }
`;
