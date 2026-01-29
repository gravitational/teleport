/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

import { useQuery } from '@tanstack/react-query';
import React, { useEffect, useState } from 'react';

import { Alert, Box, Flex, Indicator, Text } from 'design';
import { useInfoGuide } from 'shared/components/SlidingSidePanel/InfoGuide';

import {
  FeatureBox,
  FeatureHeader,
  FeatureHeaderTitle,
} from 'teleport/components/Layout';
import cfg from 'teleport/config';
import api from 'teleport/services/api';
import {
  ClusterMaintenanceInfo,
  GroupAction,
  ManagedUpdatesDetails,
} from 'teleport/services/managedUpdates';
import useTeleport from 'teleport/useTeleport';

import {
  MissingPermissionsBanner,
  POLLING_INTERVAL_MS,
  SUPPORT_URL,
} from '../shared';
import { ClientToolsCard, RolloutCard } from './Cards';
import { GroupDetailsPanel } from './GroupDetailsPanel';
import { checkIsConfigured, getMissingPermissions } from './utils';

export interface ClusterMaintenanceCardProps {
  data: ClusterMaintenanceInfo;
}

export interface ManagedUpdatesProps {
  /**
   * Cluster maintenance card component. This is used by Cloud.
   */
  ClusterMaintenanceCard?: React.ComponentType<ClusterMaintenanceCardProps>;
}

export function ManagedUpdates({
  ClusterMaintenanceCard,
}: ManagedUpdatesProps) {
  const ctx = useTeleport();
  const acl = ctx.storeUser.state.acl;

  const canReadConfig = acl.autoUpdateConfig.read;
  const canReadVersion = acl.autoUpdateVersion.read;
  const canReadRollout = acl.autoUpdateAgentRollout.read;
  const canUpdateRollout = acl.autoUpdateAgentRollout.edit;

  const canViewAnything = canReadConfig || canReadVersion || canReadRollout;
  const canViewTools = canReadConfig && canReadVersion;
  const canViewRollout = canReadRollout;

  const missingPermissions = getMissingPermissions({
    canReadConfig,
    canReadVersion,
    canReadRollout,
  });

  const { data, isLoading, isError, error, dataUpdatedAt, refetch } = useQuery({
    queryKey: ['managed-updates'],
    queryFn: () =>
      api.get(cfg.getManagedUpdatesUrl()) as Promise<ManagedUpdatesDetails>,
    enabled: canViewAnything,
    refetchInterval: canViewAnything ? POLLING_INTERVAL_MS : false,
  });

  const [selectedGroupName, setSelectedGroupName] = useState<string>(null);
  const [actionError, setActionError] = useState<string>(null);
  const { setInfoGuideConfig } = useInfoGuide();

  const selectedGroup =
    data?.groups?.find(g => g.name === selectedGroupName) || null;

  const lastSyncedTime = dataUpdatedAt ? new Date(dataUpdatedAt) : null;

  useEffect(() => {
    if (selectedGroup && data?.rollout) {
      setInfoGuideConfig({
        title: 'Progress Details',
        guide: (
          <GroupDetailsPanel
            group={selectedGroup}
            rollout={data.rollout}
            orphanedAgentVersionCounts={
              selectedGroup.isCatchAll
                ? data.orphanedAgentVersionCounts
                : undefined
            }
          />
        ),
        id: selectedGroup.name,
        panelWidth: 350,
        onClose: () => setSelectedGroupName(null),
      });
    } else {
      setInfoGuideConfig(null);
    }
  }, [
    selectedGroup,
    data?.rollout,
    data?.orphanedAgentVersionCounts,
    setInfoGuideConfig,
  ]);

  const handleGroupAction = async (
    action: GroupAction,
    groupName: string,
    force?: boolean
  ) => {
    setActionError(null);
    try {
      const url = cfg.getManagedUpdatesGroupActionUrl(groupName, action);
      const body = action === 'start' ? { force: force ?? false } : {};
      await api.post(url, body);
      refetch();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to perform group action'
      );
    }
  };

  if (!canViewAnything) {
    return (
      <FeatureBox px={6}>
        <FeatureHeader>
          <FeatureHeaderTitle>Managed Updates</FeatureHeaderTitle>
        </FeatureHeader>
        <MissingPermissionsBanner missingPermissions={missingPermissions} />
        <Box>
          <Box mb={3}>
            <ClientToolsCard fullWidth hasPermission={false} />
          </Box>
          <RolloutCard
            selectedGroupName={null}
            onGroupSelect={() => {}}
            onGroupAction={async () => {}}
            onRefresh={() => {}}
            lastSyncedTime={null}
            actionError={null}
            onDismissError={() => {}}
            canUpdateRollout={false}
            hasPermission={false}
          />
        </Box>
      </FeatureBox>
    );
  }

  if (isLoading) {
    return (
      <FeatureBox px={6}>
        <FeatureHeader>
          <FeatureHeaderTitle>Managed Updates</FeatureHeaderTitle>
        </FeatureHeader>
        <Box textAlign="center" m={10}>
          <Indicator />
        </Box>
      </FeatureBox>
    );
  }

  if (isError) {
    return (
      <FeatureBox px={6}>
        <FeatureHeader>
          <FeatureHeaderTitle>Managed Updates</FeatureHeaderTitle>
        </FeatureHeader>
        <Alert kind="danger" details={error.message}>
          Failed to load managed updates details
        </Alert>
      </FeatureBox>
    );
  }

  const isConfigured = checkIsConfigured(data);
  const hasPartialPermissions = missingPermissions.length > 0;

  return (
    <FeatureBox px={6}>
      <FeatureHeader>
        <FeatureHeaderTitle>Managed Updates</FeatureHeaderTitle>
      </FeatureHeader>

      {hasPartialPermissions && (
        <MissingPermissionsBanner missingPermissions={missingPermissions} />
      )}

      {!isConfigured && cfg.isCloud && (
        <Alert
          kind="warning"
          mb={3}
          primaryAction={{
            content: 'Go to Teleport Customer Center',
            href: SUPPORT_URL,
          }}
        >
          Could not detect configuration
          <Text typography="body2" mt={1}>
            Open a Support ticket in the Teleport Customer Center to report this
            view and request assistance for next steps.
          </Text>
        </Alert>
      )}

      <Box>
        <Flex gap={3} mb={3}>
          <ClientToolsCard
            tools={data?.tools}
            fullWidth={!data?.clusterMaintenance}
            hasPermission={canViewTools}
          />
          {data?.clusterMaintenance && ClusterMaintenanceCard && (
            <ClusterMaintenanceCard data={data.clusterMaintenance} />
          )}
        </Flex>

        <RolloutCard
          rollout={data?.rollout}
          groups={data?.groups}
          orphanedAgentVersionCounts={data?.orphanedAgentVersionCounts}
          hasPermission={canViewRollout}
          selectedGroupName={selectedGroupName}
          onGroupSelect={setSelectedGroupName}
          onGroupAction={handleGroupAction}
          onRefresh={refetch}
          lastSyncedTime={lastSyncedTime}
          actionError={actionError}
          onDismissError={() => setActionError(null)}
          canUpdateRollout={canUpdateRollout}
        />
      </Box>
    </FeatureBox>
  );
}
