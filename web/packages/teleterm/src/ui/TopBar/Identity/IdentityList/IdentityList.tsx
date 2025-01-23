/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { JSX } from 'react';
import styled from 'styled-components';

import { ButtonText, Flex, Label, P3 } from 'design';
import { Logout, Refresh, ShieldCheck, ShieldWarning } from 'design/Icon';
import Link from 'design/Link';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import { ProfileStatusError } from 'teleterm/ui/components/ProfileStatusError';
import { ProfileColor } from 'teleterm/ui/services/workspacesService';
import { DeviceTrustStatus } from 'teleterm/ui/TopBar/Identity/Identity';
import { RootClusterUri } from 'teleterm/ui/uri';

import { ColorPicker } from './ColorPicker';
import {
  AddClusterItem,
  getClusterLetter,
  IdentityListItem,
  TitleAndSubtitle,
} from './IdentityListItem';

export function ActiveCluster(props: {
  activeCluster: Cluster | undefined;
  activeColor: ProfileColor;
  deviceTrustStatus: DeviceTrustStatus;
  onChangeColor(color: ProfileColor): void;
  onRefresh(): void;
  onLogout(): void;
}) {
  return (
    <>
      <Flex p={3} pb={2} justifyContent="space-between">
        <Flex flexWrap="nowrap" gap={2} flexDirection="column">
          <Flex gap={4}>
            <Flex alignItems="center" flex={1} minWidth="0" gap={2}>
              <ColorPicker
                letter={getClusterLetter(props.activeCluster)}
                color={props.activeColor}
                setColor={props.onChangeColor}
              />
              <TitleAndSubtitle
                title={props.activeCluster.name}
                subtitle={props.activeCluster.loggedInUser?.name}
              />
            </Flex>

            <Flex
              justifyContent="space-between"
              flexDirection="row"
              alignItems="flex-start"
              gap={1}
            >
              <ButtonText
                title="Refresh Session"
                size="small"
                onClick={() => props.onRefresh()}
              >
                <Refresh size="small" />
              </ButtonText>
              <ButtonText
                onClick={() => props.onLogout()}
                intent="danger"
                size="small"
              >
                Log Out
                <Logout ml="6px" size="small" />
              </ButtonText>
            </Flex>
          </Flex>
          <Flex flexWrap="wrap" gap={1} mt={1}>
            {props.activeCluster.loggedInUser?.roles.map(role => (
              <Label
                css={`
                  line-height: 20px;
                `}
                key={role}
                kind="secondary"
              >
                {role}
              </Label>
            ))}
          </Flex>
          {props.activeCluster.profileStatusError && (
            <ProfileStatusError
              error={props.activeCluster.profileStatusError}
            />
          )}
          <DeviceTrustMessage status={props.deviceTrustStatus} />
        </Flex>
      </Flex>
      <Separator />
    </>
  );
}

export function ClusterList(props: {
  clusters: Cluster[];
  onSelect(clusterUri: RootClusterUri): void;
  onLogout?(clusterUri: RootClusterUri): void;
  onAdd(): void;
}) {
  return (
    <>
      {props.clusters.map((cluster, index) => (
        <IdentityListItem
          key={cluster.uri}
          index={index}
          cluster={cluster}
          onSelect={() => props.onSelect(cluster.uri)}
          onLogout={
            props.onLogout ? () => props.onLogout(cluster.uri) : undefined
          }
        />
      ))}
      <AddClusterItem index={props.clusters.length + 1} onClick={props.onAdd} />
    </>
  );
}

function DeviceTrustMessage(props: { status: DeviceTrustStatus }) {
  let message: JSX.Element | undefined;
  switch (props.status) {
    case 'enrolled':
      message = (
        <>
          <ShieldCheck color="success.main" size="small" mb="2px" />
          <P3>Access secured with device trust.</P3>
        </>
      );
      break;
    case 'requires-enrollment':
      message = (
        <>
          <ShieldWarning color="warning.main" size="small" mb="2px" />
          <P3>
            Full access requires a trusted device.{' '}
            <Link
              href="https://goteleport.com/docs/admin-guides/access-controls/device-trust/guide/#step-22-enroll-device"
              target="_blank"
            >
              Learn how to enroll your device.
            </Link>
          </P3>
        </>
      );
      break;
    case 'none':
      break;
    default:
      props.status satisfies never;
  }

  if (message) {
    return (
      <Flex gap={1} color="text.slightlyMuted">
        {message}
      </Flex>
    );
  }
}

const Separator = styled.div`
  background: ${props => props.theme.colors.spotBackground[1]};
  height: 1px;
`;
