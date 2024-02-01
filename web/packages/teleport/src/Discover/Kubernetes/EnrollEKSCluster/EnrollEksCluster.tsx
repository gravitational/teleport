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
import { Box, ButtonSecondary, ButtonText, Text, Toggle } from 'design';
import styled from 'styled-components';
import { FetchStatus } from 'design/DataTable/types';
import { Danger } from 'design/Alert';

import useAttempt from 'shared/hooks/useAttemptNext';
import { ToolTipInfo } from 'shared/components/ToolTip';
import { getErrMessage } from 'shared/utils/errorType';

import { EksMeta, useDiscover } from 'teleport/Discover/useDiscover';
import {
  Regions,
  integrationService,
  AwsEksCluster,
} from 'teleport/services/integrations';

import { AwsRegionSelector } from 'teleport/Discover/Shared/AwsRegionSelector';
import { ConfigureIamPerms } from 'teleport/Discover/Shared/Aws/ConfigureIamPerms';
import { isIamPermError } from 'teleport/Discover/Shared/Aws/error';
import { AgentStepProps } from 'teleport/Discover/types';
import useTeleport from 'teleport/useTeleport';

import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { generateCmd } from 'teleport/Discover/Kubernetes/HelmChart/HelmChart';
import { Kube } from 'teleport/services/kube';

import { Header, ResourceKind } from '../../Shared';

import { ClustersList } from './EksClustersList';
import { ManualHelmDialog } from './ManualHelmDialog';
import { EnrollmentDialog } from './EnrollmentDialog';
import { AgentWaitingDialog } from './AgentWaitingDialog';

type TableData = {
  items: CheckedEksCluster[];
  fetchStatus: FetchStatus;
  startKey?: string;
  currRegion?: Regions;
};

const emptyTableData: TableData = {
  items: [],
  fetchStatus: 'disabled',
  startKey: '',
};

// CheckedEksCluster is a type to describe that a
// Kube cluster has been checked with the backend
// whether or not a kube server already exists for it.
export type CheckedEksCluster = AwsEksCluster & {
  kubeServerExists?: boolean;
};

type EKSClusterEnrollmentState = {
  status: 'notStarted' | 'enrolling' | 'awaitingAgent' | 'success' | 'error';
  error?: string;
};

export function EnrollEksCluster(props: AgentStepProps) {
  const { agentMeta, emitErrorEvent } = useDiscover();
  const { attempt: fetchClustersAttempt, setAttempt: setFetchClustersAttempt } =
    useAttempt('');

  const [tableData, setTableData] = useState<TableData>({
    items: [],
    startKey: '',
    fetchStatus: 'disabled',
  });
  const [selectedCluster, setSelectedCluster] = useState<CheckedEksCluster>();
  const [selectedRegion, setSelectedRegion] = useState<Regions>();
  const [confirmedCluster, setConfirmedCluster] = useState<Kube>();
  const [enrollmentState, setEnrollmentState] =
    useState<EKSClusterEnrollmentState>({
      status: 'notStarted',
    });
  const [isAppDiscoveryEnabled, setAppDiscoveryEnabled] = useState(true);
  const [isAgentWaitingDialogShown, setIsAgentWaitingDialogShown] =
    useState(false);
  const [isManualHelmDialogShown, setIsManualHelmDialogShown] = useState(false);
  const [waitingResourceId, setWaitingResourceId] = useState('');
  const ctx = useTeleport();

  const { joinToken } = useJoinTokenSuspender([
    ResourceKind.Kubernetes,
    ResourceKind.Application,
    ResourceKind.Discovery,
  ]);

  function fetchClustersWithNewRegion(region: Regions) {
    setSelectedRegion(region);
    // Clear table when fetching with new region.
    fetchClusters({ ...emptyTableData, currRegion: region });
  }

  function fetchNextPage() {
    fetchClusters({ ...tableData });
  }

  function refreshClustersList() {
    setSelectedCluster(null);
    // When refreshing, start the table back at page 1.
    fetchClusters({ ...tableData, startKey: '', items: [] });
  }

  async function fetchClusters(data: TableData) {
    const integrationName = (agentMeta as EksMeta).awsIntegration.name;

    setTableData({ ...data, fetchStatus: 'loading' });
    setFetchClustersAttempt({ status: 'processing' });

    try {
      const { clusters: fetchedEKSClusters, nextToken } =
        await integrationService.fetchEksClusters(integrationName, {
          region: data.currRegion,
          nextToken: data.startKey,
        });

      // Abort if there were no EKS clusters for the selected region.
      if (fetchedEKSClusters.length <= 0) {
        setFetchClustersAttempt({ status: 'success' });
        setTableData({ ...data, fetchStatus: 'disabled' });
        return;
      }

      // Check if fetched EKS clusters have a Kube
      // server for it, to prevent user from enrolling
      // the same cluster.
      const query = `labels["region"] == "${fetchedEKSClusters[0].region}"`;
      const existingKubeServers = await fetchKubeServers(query, 0);
      const checkedEksClusters: CheckedEksCluster[] = fetchedEKSClusters.map(
        cluster => {
          const serverExists = existingKubeServers.agents.some(k =>
            k.name.startsWith(`${cluster.name}-${cluster.region}`)
          );

          return {
            ...cluster,
            kubeServerExists: serverExists,
          };
        }
      );

      setFetchClustersAttempt({ status: 'success' });
      setTableData({
        currRegion: data.currRegion,
        startKey: nextToken,
        fetchStatus: nextToken ? '' : 'disabled',
        // concat each page fetch.
        items: [...data.items, ...checkedEksClusters],
      });
    } catch (err) {
      const errMsg = getErrMessage(err);
      setFetchClustersAttempt({ status: 'failed', statusText: errMsg });
      setTableData(data); // fallback to previous data
      emitErrorEvent(`EKS clusters fetch error: ${errMsg}`);
    }
  }

  function clear() {
    if (fetchClustersAttempt.status === 'failed') {
      setFetchClustersAttempt({ status: '' });
    }
    if (tableData.items.length > 0) {
      setTableData(emptyTableData);
    }
    if (selectedCluster) {
      setSelectedCluster(null);
    }
    setEnrollmentState({ status: 'notStarted' });
  }

  function fetchKubeServers(query: string, limit: number) {
    return ctx.kubeService.fetchKubernetes(ctx.storeUser.getClusterId(), {
      query,
      limit,
    });
  }

  async function enroll() {
    const integrationName = (agentMeta as EksMeta).awsIntegration.name;
    setEnrollmentState({ status: 'enrolling' });

    try {
      const response = await integrationService.enrollEksClusters(
        integrationName,
        {
          region: selectedRegion,
          enableAppDiscovery: isAppDiscoveryEnabled,
          joinToken: joinToken.id,
          resourceId: joinToken.internalResourceId,
          clusterNames: [selectedCluster.name],
        }
      );

      const result = response.results?.find(
        c => c.clusterName === selectedCluster.name
      );
      if (!result) {
        setEnrollmentState({
          status: 'error',
          error: `Cluster "${selectedCluster.name}" enrollment result is unknown.`,
        });
        emitErrorEvent(
          'unknown error: no results came back from enrolling the EKS cluster.'
        );
      } else if (result.error) {
        setEnrollmentState({
          status: 'error',
          error: `Cluster enrollment error: ${result.error}`,
        });
        emitErrorEvent(`failed to enroll EKS cluster: ${result.error}`);
      } else {
        setEnrollmentState({ status: 'awaitingAgent' });
        setIsAgentWaitingDialogShown(true);
        setWaitingResourceId(result.resourceId);
      }
    } catch (err) {
      setEnrollmentState({
        status: 'error',
        error: `Cluster enrollment error: ${getErrMessage(err)}.`,
      });
      emitErrorEvent(`failed to enroll EKS cluster: ${getErrMessage(err)}`);
    }
  }

  async function handleOnProceed() {
    props.updateAgentMeta({
      kube: confirmedCluster,
      resourceName: confirmedCluster.name,
    } as EksMeta);

    props.nextStep();
  }

  const hasIamPermError = isIamPermError(fetchClustersAttempt);

  const closeEnrollmentDialog = () => {
    setEnrollmentState({ status: 'notStarted' });
  };

  const enrollmentNotAllowed =
    fetchClustersAttempt.status === 'processing' ||
    !selectedCluster ||
    enrollmentState.status !== 'notStarted';

  let command = '';
  if (selectedCluster) {
    command = generateCmd({
      namespace: 'teleport-agent',
      clusterName: selectedCluster.name,
      proxyAddr: ctx.storeUser.state.cluster.publicURL,
      tokenId: joinToken.id,
      clusterVersion: ctx.storeUser.state.cluster.authVersion,
      resourceId: joinToken.internalResourceId,
      isEnterprise: ctx.isEnterprise,
      isCloud: ctx.isCloud,
      automaticUpgradesEnabled: ctx.automaticUpgradesEnabled,
      automaticUpgradesTargetVersion: ctx.automaticUpgradesTargetVersion,
      joinLabels: [...selectedCluster.labels, ...selectedCluster.joinLabels],
      disableAppDiscovery: !isAppDiscoveryEnabled,
    });
  }

  return (
    <Box maxWidth="1000px">
      <Header>Enroll an EKS Cluster</Header>
      {fetchClustersAttempt.status === 'failed' && !hasIamPermError && (
        <Danger mt={3}>{fetchClustersAttempt.statusText}</Danger>
      )}
      <Text mt={4}>
        Select the AWS Region you would like to see EKS clusters for:
      </Text>
      <AwsRegionSelector
        onFetch={fetchClustersWithNewRegion}
        onRefresh={refreshClustersList}
        clear={clear}
        disableSelector={fetchClustersAttempt.status === 'processing'}
      />
      {!hasIamPermError && tableData.currRegion && (
        <>
          <Box mb={2}>
            <Toggle
              isToggled={isAppDiscoveryEnabled}
              onToggle={() => setAppDiscoveryEnabled(!isAppDiscoveryEnabled)}
            >
              <Box ml={2} mr={1}>
                Enable Kubernetes App Discovery
              </Box>
              <ToolTipInfo>
                Teleport's Kubernetes App Discovery will automatically identify
                and enroll to Teleport HTTP applications running inside a
                Kubernetes cluster.
              </ToolTipInfo>
            </Toggle>
          </Box>
          <ClustersList
            items={tableData.items}
            fetchStatus={tableData.fetchStatus}
            selectedCluster={selectedCluster}
            onSelectCluster={setSelectedCluster}
            fetchNextPage={fetchNextPage}
          />
          <StyledBox mb={5} mt={5}>
            <Text mb={2}>Automatically enroll selected EKS cluster</Text>
            <ButtonSecondary
              width="215px"
              type="submit"
              onClick={enroll}
              disabled={enrollmentNotAllowed}
              mt={2}
              mb={2}
            >
              Enroll EKS Cluster
            </ButtonSecondary>
            <Box>
              <ButtonText
                disabled={enrollmentNotAllowed}
                onClick={() => {
                  setIsManualHelmDialogShown(b => !b);
                }}
                pl={0}
              >
                Or click here to see instructions for manual enrollment
              </ButtonText>
            </Box>
          </StyledBox>
        </>
      )}
      {hasIamPermError && (
        <Box mb={5}>
          <ConfigureIamPerms
            kind="eks"
            region={tableData.currRegion}
            integrationRoleArn={agentMeta.awsIntegration.spec.roleArn}
          />
        </Box>
      )}
      {(enrollmentState.status === 'enrolling' ||
        enrollmentState.status === 'error') && (
        <EnrollmentDialog
          clusterName={selectedCluster.name}
          close={closeEnrollmentDialog}
          retry={enroll}
          error={enrollmentState.error}
          status={enrollmentState.status}
        />
      )}
      {isManualHelmDialogShown && (
        <ManualHelmDialog
          command={command}
          cancel={() => setIsManualHelmDialogShown(false)}
          confirmedCommands={() => {
            setEnrollmentState({ status: 'awaitingAgent' });
            setIsManualHelmDialogShown(false);
            setIsAgentWaitingDialogShown(true);
          }}
        />
      )}
      {isAgentWaitingDialogShown && (
        <AgentWaitingDialog
          joinResourceId={waitingResourceId || joinToken.internalResourceId}
          status={enrollmentState.status}
          clusterName={selectedCluster.name}
          updateWaitingResult={(result: Kube) => {
            setConfirmedCluster(result);
            setEnrollmentState({ status: 'success' });
          }}
          cancel={() => {
            if (enrollmentState.status != 'success') {
              setEnrollmentState({ status: 'notStarted' });
            }
            setIsAgentWaitingDialogShown(false);
          }}
          next={handleOnProceed}
        />
      )}
    </Box>
  );
}

const StyledBox = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;
