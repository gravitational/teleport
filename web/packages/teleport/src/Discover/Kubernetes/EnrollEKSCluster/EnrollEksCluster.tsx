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

import { useCallback, useState } from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonText,
  Flex,
  Link,
  Subtitle1,
  Text,
  Toggle,
} from 'design';
import { Danger } from 'design/Alert';
import { FetchStatus } from 'design/DataTable/types';
import { IconTooltip } from 'design/Tooltip';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'teleport/config';
import { generateCmd } from 'teleport/Discover/Kubernetes/SelfHosted';
import { ConfigureIamPerms } from 'teleport/Discover/Shared/Aws/ConfigureIamPerms';
import { isIamPermError } from 'teleport/Discover/Shared/Aws/error';
import { AwsRegionSelector } from 'teleport/Discover/Shared/AwsRegionSelector';
import {
  ConfigureDiscoveryServiceDirections,
  CreatedDiscoveryConfigDialog,
} from 'teleport/Discover/Shared/ConfigureDiscoveryService';
import { ResourceLabelTooltip } from 'teleport/Discover/Shared/ResourceLabelTooltip';
import { AgentStepProps } from 'teleport/Discover/types';
import { EksMeta, useDiscover } from 'teleport/Discover/useDiscover';
import { ResourceLabel } from 'teleport/services/agents';
import {
  createDiscoveryConfig,
  DEFAULT_DISCOVERY_GROUP_NON_CLOUD,
  DISCOVERY_GROUP_CLOUD,
  DiscoveryConfig,
} from 'teleport/services/discovery';
import {
  AwsEksCluster,
  EnrollEksClustersResponse,
  integrationService,
  Regions,
} from 'teleport/services/integrations';
import { JoinToken } from 'teleport/services/joinToken';
import { Kube } from 'teleport/services/kube';
import {
  DiscoverEvent,
  DiscoverEventStatus,
} from 'teleport/services/userEvent';
import { useV1Fallback } from 'teleport/services/version/unsupported';
import useTeleport from 'teleport/useTeleport';

import { ActionButtons, Header, LabelsCreater } from '../../Shared';
import { AgentWaitingDialog } from './AgentWaitingDialog';
import { ClustersList } from './EksClustersList';
import { EnrollmentDialog } from './EnrollmentDialog';
import ManualHelmDialog from './ManualHelmDialog';

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
  status:
    | 'notStarted'
    | 'enrolling'
    | 'awaitingAgent'
    | 'success'
    | 'error'
    | 'alreadyExists';
  error?: string;
};

export function EnrollEksCluster(props: AgentStepProps) {
  const { agentMeta, updateAgentMeta, emitErrorEvent, emitEvent } =
    useDiscover();
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
  const [isAutoDiscoveryEnabled, setAutoDiscoveryEnabled] = useState(false);
  const [isAgentWaitingDialogShown, setIsAgentWaitingDialogShown] =
    useState(false);
  const [isManualHelmDialogShown, setIsManualHelmDialogShown] = useState(false);
  const [waitingResourceId, setWaitingResourceId] = useState('');
  const [discoveryGroupName, setDiscoveryGroupName] = useState(() =>
    cfg.isCloud ? '' : DEFAULT_DISCOVERY_GROUP_NON_CLOUD
  );
  const [autoDiscoveryCfg, setAutoDiscoveryCfg] = useState<DiscoveryConfig>();
  const { attempt: autoDiscoverAttempt, setAttempt: setAutoDiscoverAttempt } =
    useAttempt('');
  // join token will be set only if user opens ManualHelmDialog,
  // we delay it to avoid premature admin action MFA confirmation request.
  const [joinToken, setJoinToken] = useState<JoinToken>(null);
  const [customLabels, setCustomLabels] = useState<ResourceLabel[]>([]);

  // TODO(kimlisa): DELETE IN 19.0
  const { tryV1Fallback } = useV1Fallback();

  const ctx = useTeleport();

  function onSelectCluster(eks: CheckedEksCluster) {
    // when changing selected cluster, clear defined labels
    setCustomLabels([]);
    setSelectedCluster(eks);
  }

  function clearSelectedCluster() {
    setSelectedCluster(null);
    setCustomLabels([]);
  }

  function fetchClustersWithNewRegion(region: Regions) {
    setSelectedRegion(region);
    // Clear table when fetching with new region.
    fetchClusters({ ...emptyTableData, currRegion: region });
  }

  function fetchNextPage() {
    fetchClusters({ ...tableData });
  }

  function refreshClustersList() {
    clearSelectedCluster();
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
    clearSelectedCluster();
    setEnrollmentState({ status: 'notStarted' });
  }

  function fetchKubeServers(query: string, limit: number) {
    return ctx.kubeService.fetchKubernetes(ctx.storeUser.getClusterId(), {
      query,
      limit,
    });
  }

  async function enableAutoDiscovery() {
    setAutoDiscoverAttempt({ status: 'processing' });

    let discoveryConfig = autoDiscoveryCfg;
    if (!discoveryConfig) {
      try {
        discoveryConfig = await createDiscoveryConfig(
          ctx.storeUser.getClusterId(),
          {
            name: crypto.randomUUID(),
            discoveryGroup: cfg.isCloud
              ? DISCOVERY_GROUP_CLOUD
              : discoveryGroupName,
            aws: [
              {
                types: ['eks'],
                regions: [tableData.currRegion],
                tags: { '*': ['*'] },
                integration: agentMeta.awsIntegration.name,
                kubeAppDiscovery: isAppDiscoveryEnabled,
              },
            ],
          }
        );
        setAutoDiscoveryCfg(discoveryConfig);
        emitEvent(
          { stepStatus: DiscoverEventStatus.Success },
          {
            eventName: DiscoverEvent.CreateDiscoveryConfig,
          }
        );
      } catch (err) {
        const message = getErrMessage(err);
        setAutoDiscoverAttempt({
          status: 'failed',
          statusText: `failed to create discovery config: ${message}`,
        });

        emitErrorEvent(`failed to create discovery config: ${message}`);
      }
    }

    setAutoDiscoverAttempt({ status: 'success' });
    updateAgentMeta({
      ...agentMeta,
      autoDiscovery: {
        config: discoveryConfig,
      },
      awsRegion: tableData.currRegion,
    } as EksMeta);
  }

  function showManualHelmDialog(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    setIsManualHelmDialogShown(true);
  }

  async function enrollWithValidation(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    return enroll();
  }

  async function enroll() {
    const integrationName = (agentMeta as EksMeta).awsIntegration.name;
    setEnrollmentState({ status: 'enrolling' });

    try {
      const req = {
        region: selectedRegion,
        enableAppDiscovery: isAppDiscoveryEnabled,
        clusterNames: [selectedCluster.name],
        extraLabels: customLabels,
      };
      let response: EnrollEksClustersResponse;
      try {
        response = await integrationService.enrollEksClustersV2(
          integrationName,
          req
        );
      } catch (err) {
        response = await tryV1Fallback({
          kind: 'enroll-eks',
          err,
          integrationName,
          req,
        });
      }

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
        const errorState: EKSClusterEnrollmentState = {
          status: 'error',
          error: `Cluster enrollment error: ${result.error}`,
        };
        if (
          result.error.includes(
            'teleport-kube-agent is already installed on the cluster'
          )
        ) {
          errorState.status = 'alreadyExists';
        }
        setEnrollmentState(errorState);
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
    updateAgentMeta({
      ...props.agentMeta,
      kube: confirmedCluster,
      resourceName: confirmedCluster.name,
    } as EksMeta);

    props.nextStep();
  }

  const hasIamPermError = isIamPermError(fetchClustersAttempt);
  const showContent =
    !hasIamPermError &&
    tableData.currRegion &&
    fetchClustersAttempt.status === 'success';

  // (Temp)
  // Self hosted auto enroll is different from cloud.
  // For cloud, we already run the discovery service for customer.
  // For on-prem, user has to run their own discovery service.
  // We hide the clusters table for on-prem if they are wanting auto discover
  // because it takes up so much space to give them instructions.
  // Future work will simply provide user a script so we can show the table then.
  const showTable = cfg.isCloud || !isAutoDiscoveryEnabled;

  const closeEnrollmentDialog = () => {
    setEnrollmentState({ status: 'notStarted' });
  };

  const enrollmentNotAllowed =
    fetchClustersAttempt.status === 'processing' ||
    !selectedCluster ||
    enrollmentState.status !== 'notStarted';

  const setJoinTokenAndGetCommand = useCallback(
    (token: JoinToken) => {
      setJoinToken(token);
      return generateCmd({
        namespace: 'teleport-agent',
        clusterName: selectedCluster.name,
        proxyAddr: ctx.storeUser.state.cluster.publicURL,
        clusterVersion: ctx.storeUser.state.cluster.authVersion,
        tokenId: token.id,
        resourceId: token.internalResourceId,
        isEnterprise: ctx.isEnterprise,
        isCloud: ctx.isCloud,
        automaticUpgradesEnabled: ctx.automaticUpgradesEnabled,
        automaticUpgradesTargetVersion: ctx.automaticUpgradesTargetVersion,
        // The labels from the `selectedCluster` are AWS tags which
        // will be imported as is. `joinLabels` are internal Teleport labels
        // added to each cluster when listing clusters.
        joinLabels: [
          ...selectedCluster.labels,
          ...selectedCluster.joinLabels,
          ...customLabels,
        ],
        disableAppDiscovery: !isAppDiscoveryEnabled,
      });
    },
    [
      ctx.automaticUpgradesEnabled,
      ctx.automaticUpgradesTargetVersion,
      ctx.isCloud,
      ctx.isEnterprise,
      ctx.storeUser.state.cluster,
      isAppDiscoveryEnabled,
      selectedCluster,
      customLabels,
    ]
  );

  return (
    <Box maxWidth="1000px">
      <Header>Enroll an EKS Cluster</Header>
      {fetchClustersAttempt.status === 'failed' && !hasIamPermError && (
        <Danger mt={3}>{fetchClustersAttempt.statusText}</Danger>
      )}
      <Text mt={4}>
        <b>Note:</b> EKS enrollment will work only with clusters that have
        access entries authentication mode enabled, see{' '}
        <Link
          href="https://docs.aws.amazon.com/eks/latest/userguide/access-entries.html#authentication-modes"
          target="_blank"
          color="text.main"
        >
          documentation.
        </Link>
      </Text>
      <Text mt={1}>
        Select the AWS Region you would like to see EKS clusters for:
      </Text>
      <AwsRegionSelector
        onFetch={fetchClustersWithNewRegion}
        onRefresh={refreshClustersList}
        clear={clear}
        disableSelector={fetchClustersAttempt.status === 'processing'}
      />
      {showContent && (
        <>
          <Box mb={2}>
            <Toggle
              isToggled={isAppDiscoveryEnabled}
              onToggle={() => setAppDiscoveryEnabled(isEnabled => !isEnabled)}
            >
              <Box ml={2} mr={1}>
                Enable Kubernetes App Discovery
              </Box>
              <IconTooltip>
                Teleport's Kubernetes App Discovery will automatically identify
                and enroll to Teleport HTTP applications running inside a
                Kubernetes cluster.
              </IconTooltip>
            </Toggle>
            <Toggle
              isToggled={isAutoDiscoveryEnabled}
              onToggle={() => setAutoDiscoveryEnabled(isEnabled => !isEnabled)}
            >
              <Box ml={2} mr={1}>
                Auto-enroll all EKS clusters for selected region
              </Box>
              <IconTooltip>
                Auto-enroll will automatically identify all EKS clusters from
                the selected region and register them as Kubernetes resources in
                your infrastructure.
              </IconTooltip>
            </Toggle>
          </Box>
          {showTable && (
            <ClustersList
              items={tableData.items}
              autoDiscovery={isAutoDiscoveryEnabled}
              fetchStatus={tableData.fetchStatus}
              selectedCluster={selectedCluster}
              onSelectCluster={onSelectCluster}
              fetchNextPage={fetchNextPage}
            />
          )}
          {!cfg.isCloud && isAutoDiscoveryEnabled && (
            <ConfigureDiscoveryServiceDirections
              clusterPublicUrl={ctx.storeUser.state.cluster.publicURL}
              discoveryGroupName={discoveryGroupName}
              setDiscoveryGroupName={setDiscoveryGroupName}
            />
          )}
          {!isAutoDiscoveryEnabled && (
            <Validation>
              {({ validator }) => (
                <>
                  {selectedCluster && (
                    <>
                      <Flex alignItems="center" gap={1} mb={2} mt={4}>
                        <Subtitle1>Optionally Add More Labels</Subtitle1>
                        <ResourceLabelTooltip
                          toolTipPosition="top"
                          resourceKind="eks"
                        />
                      </Flex>
                      <LabelsCreater
                        labels={customLabels}
                        setLabels={setCustomLabels}
                        isLabelOptional={true}
                        disableBtns={enrollmentNotAllowed}
                        noDuplicateKey={true}
                      />
                    </>
                  )}
                  <StyledBox mb={5} mt={5}>
                    <Text mb={2}>
                      Automatically enroll selected EKS cluster
                    </Text>
                    <Flex
                      alignItems="center"
                      flexDirection="column"
                      width="200px"
                    >
                      <ButtonPrimary
                        width="215px"
                        type="submit"
                        onClick={() => enrollWithValidation(validator)}
                        disabled={enrollmentNotAllowed}
                        mt={2}
                        mb={2}
                      >
                        Enroll EKS Cluster
                      </ButtonPrimary>
                      <Box>
                        <ButtonText
                          width="215px"
                          disabled={enrollmentNotAllowed}
                          onClick={() => showManualHelmDialog(validator)}
                        >
                          Or enroll manually
                        </ButtonText>
                      </Box>
                    </Flex>
                  </StyledBox>
                </>
              )}
            </Validation>
          )}
          {isAutoDiscoveryEnabled && (
            <ActionButtons
              onProceed={enableAutoDiscovery}
              disableProceed={
                fetchClustersAttempt.status === 'processing' ||
                fetchClustersAttempt.status === 'failed' ||
                (!isAutoDiscoveryEnabled && !selectedCluster) ||
                hasIamPermError ||
                (!cfg.isCloud && !discoveryGroupName)
              }
            />
          )}
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
        enrollmentState.status === 'error' ||
        enrollmentState.status === 'alreadyExists') && (
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
          setJoinTokenAndGetCommand={setJoinTokenAndGetCommand}
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
          joinResourceId={waitingResourceId || joinToken?.internalResourceId}
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
      {autoDiscoverAttempt.status !== '' && (
        <CreatedDiscoveryConfigDialog
          attempt={autoDiscoverAttempt}
          next={props.nextStep}
          close={() => setAutoDiscoverAttempt({ status: '' })}
          retry={enableAutoDiscovery}
          region={tableData.currRegion}
          notifyAboutDelay={true}
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
