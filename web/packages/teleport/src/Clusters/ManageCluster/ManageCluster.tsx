import { useState } from 'react';
import styled from 'styled-components';

import { MultiRowBox, Row } from 'design/MultiRowBox';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import { Clock, Edit } from 'design/Icon';
import Text, { H2 } from 'design/Text';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import { DataItem, IconBox } from 'teleport/Support/Support';
import { useTeleport } from 'teleport/index';
import cfg from 'teleport/config';
import { useNoMinWidth } from 'teleport/Main';

import { Contacts } from './Contacts';

export function ManageCluster() {
  // TODO: use cluster ID from path:
  // const { clusterId } = useParams<{
  //   clusterId: string;
  // }>();
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  useNoMinWidth();

  return (
    <FeatureBox maxWidth="2000px">
      <FeatureHeader alignItems="center">
        <FeatureHeaderTitle>Manage Clusters / localhost</FeatureHeaderTitle>
      </FeatureHeader>
      {/* TODO: make flex column on mobile */}
      <Flex gap="3">
        {/* cluster information */}
        <StyledMultiRowBox
          mb={3}
          style={{ flexBasis: '100%' }}
          maxWidth="454px"
        >
          <StyledRow>
            <Flex alignItems="center" justifyContent="start">
              <IconBox>
                <Icons.Cluster />
              </IconBox>
              <H2>Cluster Information</H2>
            </Flex>
          </StyledRow>
          <StyledRow
            css={`
              padding-left: ${props => props.theme.space[6]}px;
            `}
          >
            <DataItem title="Cluster Name" data={cluster.clusterId} />
            <DataItem title="Teleport Version" data={cluster.authVersion} />
            <DataItem title="Public Address" data={cluster.publicURL} />
            {cfg.tunnelPublicAddress && (
              <DataItem
                title="Public SSH Tunnel"
                data={cfg.tunnelPublicAddress}
              />
            )}
            {cfg.edition === 'ent' &&
              !cfg.isCloud &&
              cluster.licenseExpiryDateText && (
                <DataItem
                  title="License Expiry"
                  data={cluster.licenseExpiryDateText}
                />
              )}
          </StyledRow>
        </StyledMultiRowBox>
        {cfg.isCloud && <ScheduledUpgrades />}
      </Flex>
      {/* TODO: only show on enterprise Cloud or Dashboard */}
      <Contacts />
    </FeatureBox>
  );
}

export const StyledMultiRowBox = styled(MultiRowBox)`
  @media screen and (max-width: ${props => props.theme.breakpoints.mobile}px) {
    border: none;
  }
`;

export const StyledRow = styled(Row)`
  @media screen and (max-width: ${props => props.theme.breakpoints.mobile}px) {
    border: none !important;
    padding-left: 0;
    padding-bottom: 0;
  }
`;

// TODO: should this be on enterprise?
function ScheduledUpgrades() {
  const selectedUpgradeWindowStart = 16;
  const [scheduleUpgradesVisible, setScheduleUpgradesVisible] = useState(false);
  function showScheduleUpgrade() {
    setScheduleUpgradesVisible(true);
  }
  return (
    <>
      <StyledMultiRowBox
        mb={3}
        css={`
          flex-basis: 100%;
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
                <EditLink onClick={showScheduleUpgrade} ml="2" size="medium" />
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
            Changing this value changes it for everyone in your organization.
          </Text>
        </StyledRow>
      </StyledMultiRowBox>
      {scheduleUpgradesVisible && (
        <div>editing</div>
        // TODO modal
        // <ScheduleUpgrades
        //   onSave={onUpdate}
        //   onCancel={closeScheduleUpgrade}
        //   selectedWindow={selectedUpgradeWindowStart}
        //   onSelectedWindowChange={setSelectedUpgradeWindowStart}
        //   attempt={attempt}
        // />
      )}
    </>
  );
}

const makeLabel = (startHour: number): string => {
  return `${String(startHour).padStart(2, '0')}:00 (UTC)`;
};

const EditLink = styled(Edit)`
  color: ${props => props.theme.colors.text.slightlyMuted};
  &:hover,
  &:focus {
    color: ${props => props.theme.colors.text.main};
    cursor: pointer;
  }
`;
