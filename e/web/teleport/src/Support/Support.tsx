import styled from 'styled-components';

import { Flex, Text } from 'design';
import { Clock, Edit } from 'design/Icon';
import { H2 } from 'design/Text';

import {
  makeLabel,
  ScheduleUpgrades,
} from 'e-teleport/Clusters/ManageCluster/ScheduleUpgrades/ScheduleUpgrades';
import { useUpgradeWindowStart } from 'e-teleport/Clusters/ManageCluster/useUpgradeWindowStart';
import useTeleportE from 'e-teleport/useTeleportE';
import { ExternalAuditStorageCta } from 'teleport/components/ExternalAuditStorageCta';
import cfg from 'teleport/config';
import { Support } from 'teleport/Support';
import {
  DataItem,
  IconBox,
  MobileSeparator,
  StyledMultiRowBox,
  StyledRow,
} from 'teleport/Support/Support';

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
