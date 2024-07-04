/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useState } from 'react';
import { Box, Text } from 'design';

import { useDiscover } from 'teleport/Discover/useDiscover';
import useTeleport from 'teleport/useTeleport';

import { SelfHostedAutoDiscoverDirections } from 'teleport/Discover/Shared/AutoDiscovery/SelfHostedAutoDiscoverDirections';
import { DEFAULT_DISCOVERY_GROUP_NON_CLOUD } from 'teleport/services/discovery';

import { ActionButtons, Header } from '../../Shared';
import { SingleEc2InstanceInstallation } from '../Shared';

export function ConfigureDiscoveryService() {
  const { nextStep, prevStep, agentMeta, updateAgentMeta } = useDiscover();

  const [discoveryGroupName, setDiscoveryGroupName] = useState(
    DEFAULT_DISCOVERY_GROUP_NON_CLOUD
  );

  const { storeUser } = useTeleport();

  function handleNextStep() {
    updateAgentMeta({
      ...agentMeta,
      autoDiscovery: {
        config: { name: '', aws: [], discoveryGroup: discoveryGroupName },
      },
    });
    nextStep();
  }

  return (
    <Box maxWidth="1000px">
      <Header>Configure Teleport Discovery Service</Header>
      <Text mb={4}>
        The Teleport Discovery Service can connect to Amazon EC2 and
        automatically discover and enroll EC2 instances.
      </Text>
      <SingleEc2InstanceInstallation />
      <SelfHostedAutoDiscoverDirections
        showSubHeader={false}
        clusterPublicUrl={storeUser.state.cluster.publicURL}
        discoveryGroupName={discoveryGroupName}
        setDiscoveryGroupName={setDiscoveryGroupName}
      />
      <ActionButtons onProceed={handleNextStep} onPrev={prevStep} />
    </Box>
  );
}
