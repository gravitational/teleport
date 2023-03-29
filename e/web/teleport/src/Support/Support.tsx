import React from 'react';
import styled from 'styled-components';
import { Edit } from 'design/Icon';
import Text from 'design/Text';
import Support from 'teleport/Support';
import cfg from 'teleport/config';

import { DataContainer, DataItem } from 'teleport/Support/Support';

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
              <span>
                {makeLabel(selectedUpgradeWindowStart)}
                <EditLink onClick={showScheduleUpgrade} ml="2" />
              </span>
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
    </>
  );
};

const EditLink = styled(Edit)`
  color: ${props => props.theme.colors.light};
  ${props => props.theme.typography.body2}
  &:hover, &:focus {
    background: ${props => props.theme.colors.levels.elevated};
    cursor: pointer;
  }
`;

export type Props = { isCloud: boolean } & ReturnType<
  typeof useUpgradeWindowStart
>;
