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

import { useLayoutEffect } from 'react';

import Flex from 'design/Flex';
import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';
import { Cluster } from 'gen-proto-ts/teleport/lib/teleterm/v1/cluster_pb';

import {
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';
import { MockAppContextProvider } from 'teleterm/ui/fixtures/MockAppContextProvider';
import { MockAppContext } from 'teleterm/ui/fixtures/mocks';
import { RootClusterUri } from 'teleterm/ui/uri';

import { IdentityContainer } from './Identity';

export default {
  title: 'Teleterm/Identity',
};

const clusterOrange = makeRootCluster({
  name: 'orange',
  loggedInUser: makeLoggedInUser({
    name: 'bob',
    roles: ['access', 'editor'],
    sshLogins: ['root'],
  }),
  uri: '/clusters/orange',
});
const clusterViolet = makeRootCluster({
  name: 'violet',
  loggedInUser: makeLoggedInUser({ name: 'sammy' }),
  uri: '/clusters/violet',
});
const clusterGreen = makeRootCluster({
  name: 'green',
  loggedInUser: undefined,
  uri: '/clusters/green',
});

const profileStatusError =
  'No YubiKey device connected with serial number 14358031. Connect the device and try again.';

const OpenIdentityPopover = (props: {
  clusters: Cluster[];
  activeClusterUri: RootClusterUri | undefined;
}) => {
  const ctx = new MockAppContext();
  props.clusters.forEach(c => {
    ctx.addRootCluster(c);
  });
  ctx.workspacesService.setState(draftState => {
    draftState.rootClusterUri = props.activeClusterUri;
  });
  useOpenPopover();

  return (
    <Flex justifyContent="end" height="40px">
      <MockAppContextProvider appContext={ctx}>
        <IdentityContainer />
      </MockAppContextProvider>
    </Flex>
  );
};

const useOpenPopover = () => {
  useLayoutEffect(() => {
    const isProfileSelectorOpen = !!document.querySelector(
      'button[title~="logout"i]'
    );

    if (isProfileSelectorOpen) {
      return;
    }

    const button = document.querySelector(
      'button[title~="profiles"i]'
    ) as HTMLButtonElement;

    button?.click();
  }, []);
};

export function NoRootClusters() {
  return <OpenIdentityPopover clusters={[]} activeClusterUri={undefined} />;
}

export function OneClusterWithNoActiveCluster() {
  return (
    <OpenIdentityPopover
      activeClusterUri={undefined}
      clusters={[makeRootCluster({ loggedInUser: undefined })]}
    />
  );
}

export function OneClusterWithActiveCluster() {
  const cluster = makeRootCluster({
    loggedInUser: makeLoggedInUser({
      name: 'alice',
      roles: ['access', 'editor'],
      sshLogins: ['root'],
    }),
  });

  return (
    <OpenIdentityPopover clusters={[cluster]} activeClusterUri={cluster.uri} />
  );
}

export function ManyClustersWithNoActiveCluster() {
  return (
    <OpenIdentityPopover
      clusters={[clusterOrange, clusterViolet, clusterGreen]}
      activeClusterUri={undefined}
    />
  );
}

export function ManyClustersWithActiveCluster() {
  return (
    <OpenIdentityPopover
      clusters={[clusterOrange, clusterViolet, clusterGreen]}
      activeClusterUri={clusterOrange.uri}
    />
  );
}

export function ManyClustersWithProfileErrorsAndActiveCluster() {
  return (
    <OpenIdentityPopover
      clusters={[
        makeRootCluster({ ...clusterOrange, profileStatusError }),
        makeRootCluster({ ...clusterViolet, profileStatusError }),
        makeRootCluster({ ...clusterGreen, profileStatusError }),
      ]}
      activeClusterUri={clusterOrange.uri}
    />
  );
}

export function LongNamesWithManyRoles() {
  return (
    <OpenIdentityPopover
      clusters={[
        clusterOrange,
        makeRootCluster({
          ...clusterViolet,
          name: 'psv-eindhoven-eredivisie-production-lorem-ipsum',
          loggedInUser: makeLoggedInUser({
            roles: [
              'circle-mark-app-access',
              'grafana-lite-app-access',
              'grafana-gold-app-access',
              'release-lion-app-access',
              'release-fox-app-access',
              'sales-center-lorem-app-access',
              'sales-center-ipsum-db-access',
              'sales-center-shop-app-access',
              'sales-center-floor-db-access',
            ],
            name: 'ruud-van-nistelrooy-van-der-sar',
          }),
        }),
        clusterGreen,
      ]}
      activeClusterUri={clusterViolet.uri}
    />
  );
}

export function TrustedDeviceEnrolled() {
  return (
    <OpenIdentityPopover
      clusters={[
        clusterOrange,
        makeRootCluster({
          ...clusterViolet,
          loggedInUser: makeLoggedInUser({
            isDeviceTrusted: true,
            roles: ['circle-mark-app-access', 'grafana-lite-app-access'],
            sshLogins: ['root'],
          }),
        }),
      ]}
      activeClusterUri={clusterViolet.uri}
    />
  );
}

export function TrustedDeviceRequiredButNotEnrolled() {
  return (
    <OpenIdentityPopover
      clusters={[
        clusterOrange,
        makeRootCluster({
          ...clusterViolet,
          loggedInUser: makeLoggedInUser({
            trustedDeviceRequirement: TrustedDeviceRequirement.REQUIRED,
            roles: ['circle-mark-app-access', 'grafana-lite-app-access'],
            sshLogins: ['root'],
          }),
        }),
      ]}
      activeClusterUri={clusterViolet.uri}
    />
  );
}
