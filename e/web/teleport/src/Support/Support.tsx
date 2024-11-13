import React from 'react';
import styled from 'styled-components';
import { Flex } from 'design';
import { Clock, Edit } from 'design/Icon';
import Text, { H2 } from 'design/Text';
import { Support } from 'teleport/Support';
import cfg from 'teleport/config';

import {
  DataItem,
  IconBox,
  MobileSeparator,
  StyledMultiRowBox,
  StyledRow,
} from 'teleport/Support/Support';

import { ExternalAuditStorageCta } from 'teleport/components/ExternalAuditStorageCta';

import useTeleportE from 'e-teleport/useTeleportE';

import {
  makeLabel,
  ScheduleUpgrades,
} from './ScheduleUpgrades/ScheduleUpgrades';
import { useUpgradeWindowStart } from './useUpgradeWindowStart';

export default function Container() {
  const ctx = useTeleportE();
  const upgradeWindowsState = useUpgradeWindowStart(
    ctx,
    ctx.storeUser.getClusterId(),
    cfg.isCloud
  );

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
  updateWindowAttempt,
}: Props) => {
  return (
    <>
      {isCloud && (
        <>
          <MobileSeparator />
          <StyledMultiRowBox
            mb={3}
            css={`
              @media screen and (max-width: ${props =>
                  props.theme.breakpoints.mobile}px) {
                margin-top: 0px;
              }
            `}
          >
            <StyledRow>
              <Flex alignItems="center" justifyContent="start">
                <IconBox>
                  <Clock />
                </IconBox>
                <H2>Scheduled Upgrades</H2>
              </Flex>
            </StyledRow>
            <StyledRow css="padding-left: 40px !important;">
              <DataItem
                title="Window Start Time"
                data={
                  <Flex alignItems="center">
                    {makeLabel(selectedUpgradeWindowStart)}
                    <EditLink
                      onClick={showScheduleUpgrade}
                      ml="2"
                      size="medium"
                    />
                  </Flex>
                }
              />
              <Text
                typography="body2"
                css={`
                  @media screen and (max-width: ${props =>
                      props.theme.breakpoints.mobile}px) {
                    margin-left: ${props => props.theme.space[2]}px;
                  }
                `}
              >
                Window Start Time is the hour in which an upgrade may begin.
                Changing this value changes it for everyone in your
                organization.
              </Text>
            </StyledRow>
          </StyledMultiRowBox>
        </>
      )}
      {scheduleUpgradesVisible && (
        <ScheduleUpgrades
          onSave={onUpdate}
          onCancel={closeScheduleUpgrade}
          selectedWindow={selectedUpgradeWindowStart}
          onSelectedWindowChange={setSelectedUpgradeWindowStart}
          attempt={updateWindowAttempt}
        />
      )}
      <ExternalAuditStorageCta />
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
