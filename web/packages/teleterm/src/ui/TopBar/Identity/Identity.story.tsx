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

import { useEffect, useRef } from 'react';

import Flex from 'design/Flex';
import { TrustedDeviceRequirement } from 'gen-proto-ts/teleport/legacy/types/trusted_device_requirement_pb';

import {
  makeLoggedInUser,
  makeRootCluster,
} from 'teleterm/services/tshd/testHelpers';

import { Identity, IdentityHandler, IdentityProps } from './Identity';
import { IdentityRootCluster } from './useIdentity';

export default {
  title: 'Teleterm/Identity',
};

const makeTitle = (userWithClusterName: string) => userWithClusterName;
const profileStatusError =
  'No YubiKey device connected with serial number 14358031. Connect the device and try again.';

const OpenedIdentity = (props: IdentityProps) => {
  const ref = useRef<IdentityHandler>();
  useEffect(() => {
    if (ref.current) {
      ref.current.togglePopover();
    }
  }, [ref.current]);

  return (
    <Flex justifyContent="end" height="40px">
      <Identity ref={ref} {...props} />
    </Flex>
  );
};

export function NoRootClusters() {
  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={undefined}
      rootClusters={[]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}

export function OneClusterWithNoActiveCluster() {
  const identityRootCluster: IdentityRootCluster = {
    active: false,
    clusterName: 'teleport-localhost',
    userName: '',
    uri: '/clusters/localhost',
    connected: false,
    profileStatusError: '',
  };

  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={undefined}
      rootClusters={[identityRootCluster]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}

export function OneClusterWithActiveCluster() {
  const identityRootCluster: IdentityRootCluster = {
    active: true,
    clusterName: 'Teleport-Localhost',
    userName: 'alice',
    uri: '/clusters/localhost',
    connected: true,
    profileStatusError: '',
  };

  const cluster = makeRootCluster({
    uri: '/clusters/localhost',
    name: 'teleport-localhost',
    proxyHost: 'localhost:3080',
    loggedInUser: makeLoggedInUser({
      name: 'alice',
      roles: ['access', 'editor'],
      sshLogins: ['root'],
    }),
  });

  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={cluster}
      rootClusters={[identityRootCluster]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}

export function ManyClustersWithNoActiveCluster() {
  const identityRootCluster1: IdentityRootCluster = {
    active: false,
    clusterName: 'orange',
    userName: 'bob',
    uri: '/clusters/orange',
    connected: true,
    profileStatusError: '',
  };
  const identityRootCluster2: IdentityRootCluster = {
    active: false,
    clusterName: 'violet',
    userName: 'sammy',
    uri: '/clusters/violet',
    connected: true,
    profileStatusError: '',
  };
  const identityRootCluster3: IdentityRootCluster = {
    active: false,
    clusterName: 'green',
    userName: '',
    uri: '/clusters/green',
    connected: true,
    profileStatusError: '',
  };

  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={undefined}
      rootClusters={[
        identityRootCluster1,
        identityRootCluster2,
        identityRootCluster3,
      ]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}

export function ManyClustersWithActiveCluster() {
  const identityRootCluster1: IdentityRootCluster = {
    active: false,
    clusterName: 'orange',
    userName: 'bob',
    uri: '/clusters/orange',
    connected: true,
    profileStatusError: '',
  };
  const identityRootCluster2: IdentityRootCluster = {
    active: true,
    clusterName: 'violet',
    userName: 'sammy',
    uri: '/clusters/violet',
    connected: true,
    profileStatusError: '',
  };
  const identityRootCluster3: IdentityRootCluster = {
    active: false,
    clusterName: 'green',
    userName: '',
    uri: '/clusters/green',
    connected: true,
    profileStatusError: '',
  };

  const activeIdentityRootCluster = identityRootCluster2;
  const activeCluster = makeRootCluster({
    uri: activeIdentityRootCluster.uri,
    name: activeIdentityRootCluster.clusterName,
    proxyHost: 'localhost:3080',
    loggedInUser: makeLoggedInUser({
      name: activeIdentityRootCluster.userName,
      roles: ['access', 'editor'],
      sshLogins: ['root'],
    }),
  });

  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={activeCluster}
      rootClusters={[
        identityRootCluster1,
        identityRootCluster2,
        identityRootCluster3,
      ]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}

export function ManyClustersWithProfileErrorsAndActiveCluster() {
  const identityRootCluster1: IdentityRootCluster = {
    active: false,
    clusterName: 'orange',
    userName: 'bob',
    uri: '/clusters/orange',
    connected: false,
    profileStatusError: profileStatusError,
  };
  const identityRootCluster2: IdentityRootCluster = {
    active: true,
    clusterName: 'violet',
    userName: 'sammy',
    uri: '/clusters/violet',
    connected: true,
    profileStatusError: '',
  };
  const identityRootCluster3: IdentityRootCluster = {
    active: false,
    clusterName: 'green',
    userName: '',
    uri: '/clusters/green',
    connected: false,
    profileStatusError: profileStatusError,
  };

  const activeIdentityRootCluster = identityRootCluster2;
  const activeCluster = makeRootCluster({
    uri: activeIdentityRootCluster.uri,
    name: activeIdentityRootCluster.clusterName,
    proxyHost: 'localhost:3080',
    loggedInUser: makeLoggedInUser({
      name: activeIdentityRootCluster.userName,
      roles: ['access', 'editor'],
      sshLogins: ['root'],
    }),
  });

  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={activeCluster}
      rootClusters={[
        identityRootCluster1,
        identityRootCluster2,
        identityRootCluster3,
      ]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}

export function LongNamesWithManyRoles() {
  const identityRootCluster1: IdentityRootCluster = {
    active: false,
    clusterName: 'orange',
    userName: 'bob',
    uri: '/clusters/orange',
    connected: true,
    profileStatusError: '',
  };
  const identityRootCluster2: IdentityRootCluster = {
    active: true,
    clusterName: 'psv-eindhoven-eredivisie-production-lorem-ipsum',
    userName: 'ruud-van-nistelrooy-van-der-sar',
    uri: '/clusters/psv',
    connected: true,
    profileStatusError: '',
  };
  const identityRootCluster3: IdentityRootCluster = {
    active: false,
    clusterName: 'green',
    userName: '',
    uri: '/clusters/green',
    connected: true,
    profileStatusError: '',
  };

  const activeIdentityRootCluster = identityRootCluster2;
  const activeCluster = makeRootCluster({
    uri: activeIdentityRootCluster.uri,
    name: activeIdentityRootCluster.clusterName,
    proxyHost: 'localhost:3080',
    loggedInUser: makeLoggedInUser({
      name: activeIdentityRootCluster.userName,
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
      sshLogins: ['root'],
    }),
  });

  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={activeCluster}
      rootClusters={[
        identityRootCluster1,
        identityRootCluster2,
        identityRootCluster3,
      ]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}

export function TrustedDeviceEnrolled() {
  const identityRootCluster: IdentityRootCluster = {
    active: false,
    clusterName: 'orange',
    userName: 'bob',
    uri: '/clusters/orange',
    connected: true,
    profileStatusError: '',
  };

  const activeIdentityRootCluster = identityRootCluster;
  const activeCluster = makeRootCluster({
    uri: activeIdentityRootCluster.uri,
    name: activeIdentityRootCluster.clusterName,
    proxyHost: 'localhost:3080',
    loggedInUser: makeLoggedInUser({
      isDeviceTrusted: true,
      name: activeIdentityRootCluster.userName,
      roles: ['circle-mark-app-access', 'grafana-lite-app-access'],
      sshLogins: ['root'],
    }),
  });

  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={activeCluster}
      rootClusters={[identityRootCluster]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}

export function TrustedDeviceRequiredButNotEnrolled() {
  const identityRootCluster: IdentityRootCluster = {
    active: false,
    clusterName: 'orange',
    userName: 'bob',
    uri: '/clusters/orange',
    connected: true,
    profileStatusError: '',
  };

  const activeIdentityRootCluster = identityRootCluster;
  const activeCluster = makeRootCluster({
    uri: activeIdentityRootCluster.uri,
    name: activeIdentityRootCluster.clusterName,
    proxyHost: 'localhost:3080',
    loggedInUser: makeLoggedInUser({
      trustedDeviceRequirement: TrustedDeviceRequirement.REQUIRED,
      name: activeIdentityRootCluster.userName,
      roles: ['circle-mark-app-access'],
      sshLogins: ['root'],
    }),
  });

  return (
    <OpenedIdentity
      makeTitle={makeTitle}
      activeRootCluster={activeCluster}
      rootClusters={[identityRootCluster]}
      changeRootCluster={() => Promise.resolve()}
      logout={() => {}}
      addCluster={() => {}}
    />
  );
}
