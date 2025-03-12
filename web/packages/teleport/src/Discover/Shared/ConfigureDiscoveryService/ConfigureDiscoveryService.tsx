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

import { useState } from 'react';
import { Link as InternalLink } from 'react-router-dom';

import { Box, Mark, Text } from 'design';
import { OutlineInfo } from 'design/Alert/Alert';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'teleport/config';
import { useDiscover } from 'teleport/Discover/useDiscover';
import {
  createDiscoveryConfig,
  DEFAULT_DISCOVERY_GROUP_NON_CLOUD,
} from 'teleport/services/discovery';
import useTeleport from 'teleport/useTeleport';

import { ActionButtons, Header, ResourceKind } from '../../Shared';
import { ConfigureDiscoveryServiceDirections } from './ConfigureDiscoveryServiceDirections';
import { CreatedDiscoveryConfigDialog } from './CreatedDiscoveryConfigDialog';

export function ConfigureDiscoveryService({
  withCreateConfig = false,
}: {
  /**
   * if true, creates a discovery config resource when clicking on
   * nextStep button
   */
  withCreateConfig?: boolean;
}) {
  const ctx = useTeleport();
  const clusterId = ctx.storeUser.getClusterId();
  const {
    nextStep,
    prevStep,
    agentMeta,
    updateAgentMeta,
    resourceSpec,
    emitErrorEvent,
  } = useDiscover();

  const {
    attempt: createDiscoveryConfigAttempt,
    setAttempt: setCreateDiscoveryConfigAttempt,
  } = useAttempt('');

  const [discoveryGroupName, setDiscoveryGroupName] = useState(
    DEFAULT_DISCOVERY_GROUP_NON_CLOUD
  );

  const { storeUser } = useTeleport();

  function handleNextStep() {
    if (withCreateConfig) {
      createDiscoveryCfg();
      nextStep();
      return;
    }

    updateAgentMeta({
      ...agentMeta,
      autoDiscovery: {
        config: { name: '', aws: [], discoveryGroup: discoveryGroupName },
      },
    });
    nextStep();
  }

  async function createDiscoveryCfg() {
    try {
      setCreateDiscoveryConfigAttempt({ status: 'processing' });
      const discoveryConfig = await createDiscoveryConfig(clusterId, {
        name: crypto.randomUUID(),
        discoveryGroup: discoveryGroupName,
        aws: [
          {
            types: ['rds'],
            regions: [agentMeta.awsRegion],
            tags: { 'vpc-id': [agentMeta.awsVpcId] },
            integration: agentMeta.awsIntegration.name,
          },
        ],
      });
      setCreateDiscoveryConfigAttempt({ status: 'success' });
      updateAgentMeta({
        ...agentMeta,
        autoDiscovery: {
          config: discoveryConfig,
        },
      });
    } catch (err) {
      const message = getErrMessage(err);
      setCreateDiscoveryConfigAttempt({
        status: 'failed',
        statusText: `failed to create discovery config: ${message}`,
      });
      emitErrorEvent(`failed to create discovery config: ${message}`);
      return;
    }
  }

  return (
    <Box maxWidth="1000px">
      <Header>Configure Teleport Discovery Service</Header>
      <EnrollInfo kind={resourceSpec.kind} />
      <ConfigureDiscoveryServiceDirections
        showSubHeader={false}
        clusterPublicUrl={storeUser.state.cluster.publicURL}
        discoveryGroupName={discoveryGroupName}
        setDiscoveryGroupName={setDiscoveryGroupName}
      />
      <ActionButtons onProceed={handleNextStep} onPrev={prevStep} />
      {createDiscoveryConfigAttempt.status !== '' && (
        <CreatedDiscoveryConfigDialog
          attempt={createDiscoveryConfigAttempt}
          next={nextStep}
          close={() => setCreateDiscoveryConfigAttempt({ status: '' })}
          retry={handleNextStep}
          region={agentMeta.awsRegion}
          notifyAboutDelay={false}
        />
      )}
    </Box>
  );
}

function EnrollInfo({ kind }: { kind: ResourceKind }) {
  if (kind === ResourceKind.Database) {
    return (
      <Text mb={4}>
        The Teleport Discovery Service can connect to Amazon RDS and
        automatically discover and enroll RDS instances and clusters.
      </Text>
    );
  }

  if (kind === ResourceKind.Server) {
    return (
      <>
        <Text mb={4}>
          The Teleport Discovery Service can connect to Amazon EC2 and
          automatically discover and enroll EC2 instances.
        </Text>
        <OutlineInfo mt={3} linkColor="buttons.link.default">
          Auto discovery will enroll all EC2 instances found in a region. If you
          want to enroll a <Mark>single</Mark> EC2 instance instead, consider
          following{' '}
          <InternalLink
            to={{
              pathname: cfg.routes.discover,
              state: { searchKeywords: 'linux' },
            }}
          >
            the Teleport service installation flow
          </InternalLink>
          .
        </OutlineInfo>
      </>
    );
  }

  return null;
}
