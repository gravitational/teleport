import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { Box, Flex, Text } from 'design';
import { Danger } from 'design/Alert';
import { ShimmerBox } from 'design/ShimmerBox';

import {
  EnvironmentProfile,
  UpgradeWindowStartHour,
} from 'e-teleport/services/cloud/cloud';
import { ProfileInput } from 'e-teleport/Support/ScheduledUpgrades/Profile';
import { WindowInput } from 'e-teleport/Support/ScheduledUpgrades/WindowInput';
import useTeleportE from 'e-teleport/useTeleportE';
import cfg from 'teleport/config';
import {
  Card,
  CardTitle,
  DocsLink,
  InfoItem,
  ManagedUpdates as ManagedUpdatesOSS,
} from 'teleport/ManagedUpdates';
import { ClusterMaintenanceInfo } from 'teleport/services/managedUpdates';

export default function ManagedUpdates() {
  if (!cfg.isCloud) {
    return <ManagedUpdatesOSS />;
  }

  return <ManagedUpdatesOSS ClusterMaintenanceCard={ClusterMaintenanceCard} />;
}

const DOCS_URL =
  'https://goteleport.com/docs/upgrading/cloud-cluster-updates/#maintenance-windows';

function ClusterMaintenanceCard({ data }: { data: ClusterMaintenanceInfo }) {
  const ctx = useTeleportE();
  const clusterId = ctx.storeUser.getClusterId();
  const queryClient = useQueryClient();

  const [startHour, setStartHour] = useState<UpgradeWindowStartHour>(
    data.maintenanceStartHour as UpgradeWindowStartHour
  );
  const [prevMaintenanceStartHour, setPrevMaintenanceStartHour] = useState(
    data.maintenanceStartHour
  );
  if (data.maintenanceStartHour !== prevMaintenanceStartHour) {
    setPrevMaintenanceStartHour(data.maintenanceStartHour);
    setStartHour(data.maintenanceStartHour as UpgradeWindowStartHour);
  }
  const [editWindow, setEditWindow] = useState(false);
  const [env, setEnv] = useState<EnvironmentProfile>();
  const [editEnv, setEditEnv] = useState(false);

  const {
    status,
    error,
    data: envData,
  } = useQuery({
    enabled: !!clusterId,
    queryKey: ['env', clusterId],
    staleTime: 0,
    queryFn: () => ctx.cloudService.getEnvironmentProfile(),
  });

  const [prevEnvProfile, setPrevEnvProfile] = useState(
    envData?.environmentProfile
  );
  if (envData && envData.environmentProfile !== prevEnvProfile) {
    setPrevEnvProfile(envData.environmentProfile);
    setEnv(undefined);
  }

  const updateEnvMutation = useMutation({
    mutationFn: () =>
      ctx.cloudService.updateEnvironmentProfile(
        env ?? (envData!.environmentProfile as EnvironmentProfile)
      ),
    onSuccess: data => {
      queryClient.setQueryData(['env', clusterId], data);
      setEnv(undefined);
      setEditEnv(false);
    },
  });

  const updateWindowMutation = useMutation({
    mutationFn: () =>
      ctx.cloudService.updateUpgradeWindowStart(clusterId, startHour),
    onSuccess: () => setEditWindow(false),
  });

  return (
    <Card flex="1 1 50%">
      <CardTitle>Cluster Maintenance</CardTitle>
      <Flex alignItems="flex-start" gap={1} mb={3} flexDirection="column">
        <Text color="text.slightlyMuted">
          Auth and Proxy Service automatic updates will occur during the
          selected maintenance window below.
        </Text>
        <DocsLink docsUrl={DOCS_URL} />
      </Flex>
      <Box>
        {status === 'pending' && <ShimmerBox height="24px" width="100%" />}
        {status === 'error' && (
          <Danger details={error?.message || 'Unknown error occurred'}>
            Error loading
          </Danger>
        )}
        {status === 'success' && (
          <>
            <InfoItem
              label="Control Plane Version"
              value={data.controlPlaneVersion}
            />
            <WindowInput
              muted
              mutation={updateWindowMutation}
              edit={editWindow}
              setEdit={setEditWindow}
              value={startHour}
              setValue={setStartHour}
            />
            <ProfileInput
              muted
              mutation={updateEnvMutation}
              edit={editEnv}
              setEdit={setEditEnv}
              value={env ?? (envData!.environmentProfile as EnvironmentProfile)}
              setValue={setEnv}
            />
          </>
        )}
      </Box>
    </Card>
  );
}
