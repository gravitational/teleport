import { useMutation, useQueries, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';

import { Flex } from 'design';
import { Danger } from 'design/Alert';
import { Clock } from 'design/Icon';
import { ShimmerBox } from 'design/ShimmerBox';
import { H2 } from 'design/Text';
import { P } from 'design/Text/Text';

import { UpgradeWindowStartHour } from 'e-teleport/services/cloud';
import { EnvironmentProfile } from 'e-teleport/services/cloud/cloud';
import { ProfileInput } from 'e-teleport/Support/ScheduledUpgrades/Profile';
import { WindowInput } from 'e-teleport/Support/ScheduledUpgrades/WindowInput';
import useTeleportE from 'e-teleport/useTeleportE';
import { IconBox, SupportSectionCard } from 'teleport/Support/Support';

export function ScheduledUpgrades() {
  const ctx = useTeleportE();
  const { clusterId } = ctx.storeUser.state.cluster;
  const queryClient = useQueryClient();

  const [startHour, setStartHour] = useState<UpgradeWindowStartHour>();
  const [editWindow, setEditWindow] = useState(false);
  const [env, setEnv] = useState<EnvironmentProfile>();
  const [editEnv, setEditEnv] = useState(false);

  const results = useQueries({
    queries: [
      {
        queryKey: ['window', clusterId],
        queryFn: () => ctx.cloudService.getUpgradeWindowStartHour(clusterId),
      },
      {
        queryKey: ['env', clusterId],
        queryFn: () => ctx.cloudService.getEnvironmentProfile(),
      },
    ],
  });
  const [windowResp, envResp] = results;

  const [prevWindowData, setPrevWindowData] = useState(windowResp.data);
  if (windowResp.data !== undefined && windowResp.data !== prevWindowData) {
    setPrevWindowData(windowResp.data);
    setStartHour(undefined);
  }

  const [prevEnvProfile, setPrevEnvProfile] = useState(
    envResp.data?.environmentProfile
  );
  if (envResp.data && envResp.data.environmentProfile !== prevEnvProfile) {
    setPrevEnvProfile(envResp.data.environmentProfile);
    setEnv(undefined);
  }

  const updateEnvMutation = useMutation({
    mutationFn: () =>
      ctx.cloudService.updateEnvironmentProfile(
        env ?? (envResp.data!.environmentProfile as EnvironmentProfile)
      ),
    onSuccess: data => {
      queryClient.setQueryData(['env', clusterId], data);
      setEnv(undefined);
      setEditEnv(false);
    },
  });

  const updateWindowMutation = useMutation({
    mutationFn: () =>
      ctx.cloudService.updateUpgradeWindowStart(
        clusterId,
        startHour ?? (windowResp.data as UpgradeWindowStartHour)
      ),
    onSuccess: () => {
      queryClient.setQueryData(
        ['window', clusterId],
        startHour ?? windowResp.data
      );
      setStartHour(undefined);
      setEditWindow(false);
    },
  });

  return (
    <SupportSectionCard>
      <Flex alignItems="center" justifyContent="start" mb={3}>
        <IconBox>
          <Clock size={16} />
        </IconBox>
        <H2>Scheduled Upgrades</H2>
      </Flex>
      <P mb={3}>
        Window Start Time is the hour in which an upgrade may begin. The
        settings below apply to the entire organization.
      </P>
      {results.some(r => r.status === 'error') ? (
        <Danger
          details={
            windowResp?.error?.message ||
            envResp?.error?.message ||
            'Unknown error occurred'
          }
        >
          Error loading
        </Danger>
      ) : results.some(r => r.status === 'pending') ? (
        <ShimmerBox height="24px" width="100%" />
      ) : null}
      {results.every(r => r.status === 'success') && (
        <>
          <WindowInput
            mutation={updateWindowMutation}
            edit={editWindow}
            setEdit={setEditWindow}
            value={startHour ?? (windowResp.data as UpgradeWindowStartHour)}
            setValue={setStartHour}
          />
          <ProfileInput
            mutation={updateEnvMutation}
            edit={editEnv}
            setEdit={setEditEnv}
            value={
              env ?? (envResp.data!.environmentProfile as EnvironmentProfile)
            }
            setValue={setEnv}
          />
        </>
      )}
    </SupportSectionCard>
  );
}
