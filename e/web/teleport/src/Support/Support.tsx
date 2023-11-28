import React from 'react';
import styled from 'styled-components';
import { Box, Flex } from 'design';
import { Edit } from 'design/Icon';
import Text from 'design/Text';
import Support from 'teleport/Support';
import cfg from 'teleport/config';

import { DataContainer, DataItem } from 'teleport/Support/Support';

import { ExternalAuditStorageCta } from 'teleport/components/ExternalAuditStorageCta';

import useTeleportE from 'e-teleport/useTeleportE';

import {
  makeLabel,
  ScheduleUpgrades,
} from './ScheduleUpgrades/ScheduleUpgrades';
import { useUpgradeWindowStart } from './useUpgradeWindowStart';

export default function Container() {
  const ctx = useTeleportE();
  const upgradeWindowsState = useUpgradeWindowStart(ctx);

  return (
    <Support>
      <SupportE {...upgradeWindowsState} isCloud={cfg.isCloud} />
    </Support>
  );
}

export const SupportE = ({
  isCloud,
  showScheduleUpgrade,
  scheduleUpgradesVisible,
  closeScheduleUpgrade,
  onUpdate,
  selectedUpgradeWindowStart,
  setSelectedUpgradeWindowStart,
  attempt,
}: Props) => {
  return (
    <>
      {isCloud && (
        <DataContainer title="Scheduled Upgrades">
          <DataItem
            title="Window Start Time"
            data={
              <Flex alignItems="center">
                {makeLabel(selectedUpgradeWindowStart)}
                <EditLink onClick={showScheduleUpgrade} ml="2" size="medium" />
              </Flex>
            }
          />
          <Text>
            Window Start Time is the hour in which an upgrade may begin.
            Changing this value changes it for everyone in your organization.
          </Text>
        </DataContainer>
      )}
      {scheduleUpgradesVisible && (
        <ScheduleUpgrades
          onSave={onUpdate}
          onCancel={closeScheduleUpgrade}
          selectedWindow={selectedUpgradeWindowStart}
          onSelectedWindowChange={setSelectedUpgradeWindowStart}
          attempt={attempt}
        />
      )}
      <Box mt="4">
        <ExternalAuditStorageCta />
      </Box>
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

export type Props = {
  isCloud: boolean;
} & ReturnType<typeof useUpgradeWindowStart>;
