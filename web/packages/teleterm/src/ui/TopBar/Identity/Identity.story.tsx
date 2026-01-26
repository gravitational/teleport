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

import { Meta, StoryObj } from '@storybook/react-vite';
import { useLayoutEffect } from 'react';

import Flex from 'design/Flex';
import { Timestamp } from 'gen-proto-ts/google/protobuf/timestamp_pb';
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

interface StoryProps {
  clusters: ('violet' | 'orange' | 'green')[];
  activeCluster: boolean;
  activeClusterExpired: boolean;
  deviceTrust: 'enrolled' | 'required-not-enrolled' | 'not-enrolled';
  showProfileErrors: boolean;
}

const meta: Meta<StoryProps> = {
  title: 'Teleterm/Identity',
  component: props => {
    const hasOrange = props.clusters.includes('orange');
    const hasViolet = props.clusters.includes('violet');
    const hasGreen = props.clusters.includes('green');
    const clusters = [
      hasOrange &&
        makeRootCluster({
          ...clusterOrange,
          profileStatusError: props.showProfileErrors ? profileStatusError : '',
        }),
      hasViolet &&
        makeRootCluster({
          ...clusterViolet,
          profileStatusError: props.showProfileErrors ? profileStatusError : '',
        }),
      hasGreen &&
        makeRootCluster({
          ...clusterGreen,
          profileStatusError: props.showProfileErrors ? profileStatusError : '',
        }),
    ].filter(Boolean);

    const hasClusterWithLoggedInUser =
      props.activeCluster && (hasOrange || hasViolet);
    if (hasClusterWithLoggedInUser) {
      clusters[0].loggedInUser = makeLoggedInUser({
        ...clusters[0].loggedInUser,
        validUntil: Timestamp.fromDate(
          props.activeClusterExpired
            ? new Date()
            : new Date(Date.now() + 24 * 60 * 60 * 1000)
        ),
        isDeviceTrusted: props.deviceTrust === 'enrolled',
        trustedDeviceRequirement:
          props.deviceTrust === 'required-not-enrolled'
            ? TrustedDeviceRequirement.REQUIRED
            : TrustedDeviceRequirement.NOT_REQUIRED,
      });
    }

    return (
      <OpenIdentityPopover
        clusters={clusters}
        activeClusterUri={hasClusterWithLoggedInUser && clusters[0]?.uri}
      />
    );
  },
  argTypes: {
    clusters: {
      control: { type: 'check' },
      options: ['violet', 'orange', 'green'],
      description: 'List of clusters to show.',
    },
    activeCluster: {
      control: { type: 'boolean' },
      description: 'Makes "violet" or "orange" an active cluster.',
    },
    deviceTrust: {
      control: { type: 'radio' },
      options: ['enrolled', 'required-not-enrolled', 'not-enrolled'],
      description: 'Controls device trust requirement.',
    },
    activeClusterExpired: {
      control: { type: 'boolean' },
      description: 'Whether the active cluster has expired cert.',
    },
    showProfileErrors: {
      control: { type: 'boolean' },
      description: 'Shows profile errors for all clusters.',
    },
  },
  args: {
    clusters: ['violet', 'orange', 'green'],
    activeCluster: true,
    deviceTrust: 'not-enrolled',
    activeClusterExpired: false,
    showProfileErrors: false,
  },
};

export default meta;

const clusterOrange = makeRootCluster({
  name: 'orange-psv-eindhoven-eredivisie-production-lorem-ipsum',
  loggedInUser: makeLoggedInUser({
    name: 'ruud-van-nistelrooy-van-der-sar',
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
  }),
  uri: '/clusters/orange',
});
const clusterViolet = makeRootCluster({
  name: 'violet',
  loggedInUser: makeLoggedInUser({
    name: 'sammy',
    roles: ['access', 'editor'],
  }),
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
  ctx.statePersistenceService.putState({
    ...ctx.statePersistenceService.getState(),
    showTshHomeMigrationBanner: true,
  });
  props.clusters.forEach(c => {
    ctx.addRootCluster(c);
  });
  ctx.workspacesService.addWorkspace(clusterGreen.uri);
  ctx.workspacesService.addWorkspace(clusterViolet.uri);
  ctx.workspacesService.addWorkspace(clusterOrange.uri);
  ctx.workspacesService.setState(draftState => {
    draftState.rootClusterUri = props.activeClusterUri;
    draftState.workspaces[clusterGreen.uri].color = 'green';
    draftState.workspaces[clusterViolet.uri].color = 'purple';
    draftState.workspaces[clusterOrange.uri].color = 'yellow';
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

export const NoRootClusters: StoryObj<StoryProps> = {
  args: {
    clusters: [],
  },
};

export const OneClusterWithNoActiveCluster: StoryObj<StoryProps> = {
  args: {
    clusters: ['orange'],
    activeCluster: false,
  },
};

export const OneClusterWithActiveCluster: StoryObj<StoryProps> = {
  args: {
    clusters: ['violet'],
  },
};

export const ManyClustersWithNoActiveCluster: StoryObj<StoryProps> = {
  args: {
    clusters: ['orange', 'green', 'violet'],
    activeCluster: false,
  },
};

export const ManyClustersWithActiveCluster: StoryObj<StoryProps> = {
  args: {
    clusters: ['orange', 'green', 'violet'],
  },
};

export const ManyClustersWithProfileErrorsAndActiveCluster: StoryObj<StoryProps> =
  {
    args: {
      clusters: ['orange', 'green', 'violet'],
      showProfileErrors: true,
    },
  };

export const TrustedDeviceEnrolled: StoryObj<StoryProps> = {
  args: {
    deviceTrust: 'enrolled',
  },
};

export const TrustedDeviceRequiredButNotEnrolled: StoryObj<StoryProps> = {
  args: {
    deviceTrust: 'required-not-enrolled',
  },
};

export const ActiveClusterExpired: StoryObj<StoryProps> = {
  args: {
    activeClusterExpired: true,
  },
};
