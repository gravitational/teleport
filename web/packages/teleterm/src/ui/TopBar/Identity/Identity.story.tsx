/**
 * Copyright 2023 Gravitational, Inc
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useRef, useEffect } from 'react';
import Flex from 'design/Flex';

import * as tshd from 'teleterm/services/tshd/types';

import { Identity, IdentityHandler, IdentityProps } from './Identity';
import { IdentityRootCluster } from './useIdentity';

export default {
  title: 'Teleterm/Identity',
};

const makeTitle = (userWithClusterName: string) => userWithClusterName;

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
  };

  const cluster: tshd.Cluster = {
    uri: '/clusters/localhost',
    name: 'teleport-localhost',
    proxyHost: 'localhost:3080',
    connected: true,
    leaf: false,
    authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
    loggedInUser: {
      activeRequestsList: [],
      name: 'alice',
      rolesList: ['access', 'editor'],
      sshLoginsList: ['root'],
      requestableRolesList: [],
      suggestedReviewersList: [],
    },
  };

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
  };
  const identityRootCluster2: IdentityRootCluster = {
    active: false,
    clusterName: 'violet',
    userName: 'sammy',
    uri: '/clusters/violet',
    connected: true,
  };
  const identityRootCluster3: IdentityRootCluster = {
    active: false,
    clusterName: 'green',
    userName: '',
    uri: '/clusters/green',
    connected: true,
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
  };
  const identityRootCluster2: IdentityRootCluster = {
    active: true,
    clusterName: 'violet',
    userName: 'sammy',
    uri: '/clusters/violet',
    connected: true,
  };
  const identityRootCluster3: IdentityRootCluster = {
    active: false,
    clusterName: 'green',
    userName: '',
    uri: '/clusters/green',
    connected: true,
  };

  const activeIdentityRootCluster = identityRootCluster2;
  const activeCluster: tshd.Cluster = {
    uri: activeIdentityRootCluster.uri,
    name: activeIdentityRootCluster.clusterName,
    proxyHost: 'localhost:3080',
    connected: true,
    leaf: false,
    authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
    loggedInUser: {
      activeRequestsList: [],
      name: activeIdentityRootCluster.userName,
      rolesList: ['access', 'editor'],
      sshLoginsList: ['root'],
      requestableRolesList: [],
      suggestedReviewersList: [],
    },
  };

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
  };
  const identityRootCluster2: IdentityRootCluster = {
    active: true,
    clusterName: 'psv-eindhoven-eredivisie-production-lorem-ipsum',
    userName: 'ruud-van-nistelrooy-van-der-sar',
    uri: '/clusters/psv',
    connected: true,
  };
  const identityRootCluster3: IdentityRootCluster = {
    active: false,
    clusterName: 'green',
    userName: '',
    uri: '/clusters/green',
    connected: true,
  };

  const activeIdentityRootCluster = identityRootCluster2;
  const activeCluster: tshd.Cluster = {
    uri: activeIdentityRootCluster.uri,
    name: activeIdentityRootCluster.clusterName,
    proxyHost: 'localhost:3080',
    connected: true,
    leaf: false,
    authClusterId: '73c4746b-d956-4f16-9848-4e3469f70762',
    loggedInUser: {
      activeRequestsList: [],
      name: activeIdentityRootCluster.userName,
      rolesList: [
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
      sshLoginsList: ['root'],
      requestableRolesList: [],
      suggestedReviewersList: [],
    },
  };

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
