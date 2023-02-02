/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState, Suspense } from 'react';
import styled from 'styled-components';
import { Text, Box, Link, ButtonSecondary } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy/TextSelectCopyMulti';
import { CatchError } from 'teleport/components/CatchError';
import {
  useJoinToken,
  clearCachedJoinTokenResult,
} from 'teleport/Discover/Shared/JoinTokenContext';
import { Timeout } from 'teleport/Discover/Shared/Timeout';
import useTeleport from 'teleport/useTeleport';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { PollBox, PollState } from 'teleport/Discover/Shared/PollState';

import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  Mark,
  ResourceKind,
  TextIcon,
} from '../../Shared';

import type { AgentStepProps } from '../../types';
import type { JoinToken } from 'teleport/services/joinToken';
import type { AgentMeta, KubeMeta } from 'teleport/Discover/useDiscover';
import type { Kube } from 'teleport/services/kube';

export default function Container(
  props: AgentStepProps & { runJoinTokenPromise?: boolean }
) {
  const [namespace, setNamespace] = useState('');
  const [clusterName, setClusterName] = useState('');

  return (
    // This outer CatchError and Suspense handles
    // join token api fetch error and loading states.
    <CatchError
      onRetry={clearCachedJoinTokenResult}
      fallbackFn={props => (
        <Box>
          <Heading />
          <StepOne />
          <StepTwo
            error={props.error}
            onRetry={props.retry}
            namespace={namespace}
            setNamespace={setNamespace}
            clusterName={clusterName}
            setClusterName={setClusterName}
          />
          <ActionButtons onProceed={() => null} disableProceed={true} />
        </Box>
      )}
    >
      <Suspense
        fallback={
          <Box>
            <Heading />
            <StepOne />
            <StepTwo
              namespace={namespace}
              setNamespace={setNamespace}
              clusterName={clusterName}
              setClusterName={setClusterName}
            />
            <ActionButtons onProceed={() => null} disableProceed={true} />
          </Box>
        }
      >
        <DownloadScript
          {...props}
          namespace={namespace}
          setNamespace={setNamespace}
          clusterName={clusterName}
          setClusterName={setClusterName}
        />
      </Suspense>
    </CatchError>
  );
}

export function DownloadScript(
  props: AgentStepProps & {
    namespace: string;
    setNamespace(n: string): void;
    clusterName: string;
    setClusterName(c: string): void;
    // runJoinTokenPromise is used only for development
    // (eg. stories) to bypass user input requirement.
    runJoinTokenPromise?: boolean;
  }
) {
  const { runJoinTokenPromise = false } = props;
  // runPromise is a flag that when false prevents the `useJoinToken` hook
  // from running on the first run. If we let this run in the background on first run,
  // and it returns an error, the error is caught by the CatchError
  // component which will interrupt the user (rendering the failed state)
  // without the user clicking on any buttons. After the user fills out required fields
  // and clicks on `handleSubmit`, this flag will always be true.
  const [runPromise, setRunPromise] = useState(runJoinTokenPromise);
  const [showHelmChart, setShowHelmChart] = useState(false);
  const {
    joinToken,
    reloadJoinToken: fetchJoinToken,
    timeout,
  } = useJoinToken(ResourceKind.Kubernetes, runPromise);

  function handleSubmit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    setRunPromise(true);
  }

  function reloadJoinToken(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    fetchJoinToken();

    // Hide chart until we finished fetching a new join token.
    setShowHelmChart(false);
  }

  React.useEffect(() => {
    if (joinToken) {
      setShowHelmChart(true);
    }
  }, [joinToken]);

  return (
    <Box>
      <Heading />
      <StepOne />
      <StepTwo
        handleSubmit={v => (joinToken ? reloadJoinToken(v) : handleSubmit(v))}
        namespace={props.namespace}
        setNamespace={props.setNamespace}
        clusterName={props.clusterName}
        setClusterName={props.setClusterName}
        hasJoinToken={!!joinToken}
      />
      {showHelmChart ? (
        <InstallHelmChart
          namespace={props.namespace}
          clusterName={props.clusterName}
          joinToken={joinToken}
          pollingTimeout={timeout}
          nextStep={props.nextStep}
          updateAgentMeta={props.updateAgentMeta}
        />
      ) : (
        <ActionButtons onProceed={() => null} disableProceed={true} />
      )}
    </Box>
  );
}

const Heading = () => {
  return (
    <>
      <Header>Configure Resource</Header>
      <HeaderSubtitle>
        Install Teleport Service in your cluster via Helm to easily connect your
        Kubernetes cluster with Teleport.
        <br />
        For all the available values of the helm chart see the{' '}
        <Link
          href="https://goteleport.com/docs/kubernetes-access/helm/reference/teleport-kube-agent/"
          target="_blank"
        >
          the documentation
        </Link>
        .
      </HeaderSubtitle>
    </>
  );
};

const StepOne = () => {
  return (
    <StyledBox mb={5}>
      <Text bold>Step 1</Text>
      <Text typography="subtitle1" mb={3}>
        Add teleport-agent chart to your charts repository
      </Text>
      <TextSelectCopyMulti
        lines={[
          {
            text: 'helm repo add teleport https://charts.releases.teleport.dev && helm repo update',
          },
        ]}
      />
    </StyledBox>
  );
};

const StepTwo = ({
  handleSubmit,
  namespace,
  setNamespace,
  clusterName,
  setClusterName,
  hasJoinToken,
  error,
  onRetry,
}: {
  error?: Error;
  onRetry?(): void;
  handleSubmit?(v: Validator): void;
  namespace: string;
  setNamespace(n: string): void;
  clusterName: string;
  setClusterName(c: string): void;
  hasJoinToken?: boolean;
}) => {
  return (
    <StyledBox mb={5}>
      <Text bold>Step 2</Text>
      <Text typography="subtitle1" mb={3}>
        Generate a command to automatically configure and install the
        teleport-agent namespace
      </Text>
      <Validation>
        {({ validator }) => (
          <>
            <Box mb={4}>
              <FieldInput
                mb={3}
                rule={requiredField('Namespace is required')}
                label="Teleport Service Namespace"
                autoFocus
                value={namespace}
                placeholder="teleport"
                width="100%"
                mr="3"
                onChange={e => setNamespace(e.target.value)}
              />
              <FieldInput
                rule={requiredField('Kubernetes Cluster Name is required')}
                label="Kubernetes Cluster Name"
                labelTip="Name shown to Teleport users connecting to the cluster"
                value={clusterName}
                placeholder="my-cluster"
                width="100%"
                mr="3"
                onChange={e => setClusterName(e.target.value)}
              />
            </Box>
            <ButtonSecondary
              width="200px"
              type="submit"
              // Let user re-try on error
              disabled={!error && !handleSubmit}
              onClick={() => (error ? onRetry() : handleSubmit(validator))}
            >
              {hasJoinToken ? 'Regenerate Command' : 'Generate Command'}
            </ButtonSecondary>
          </>
        )}
      </Validation>
      {error && (
        <Box>
          <TextIcon mt={3}>
            <Icons.Warning ml={1} color="danger" />
            Encountered Error: {error.message}
          </TextIcon>
        </Box>
      )}
    </StyledBox>
  );
};

const generateCmd = (data: {
  namespace: string;
  clusterName: string;
  proxyAddr: string;
  tokenId: string;
  clusterVersion: string;
  resourceId: string;
}) => {
  return `cat << EOF > prod-cluster-values.yaml
roles: kube
authToken: ${data.tokenId}
proxyAddr: ${data.proxyAddr}
kubeClusterName: ${data.clusterName}
teleportVersionOverride: ${data.clusterVersion}
labels:
    teleport.internal/resource-id: ${data.resourceId}
EOF
 
helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --create-namespace --namespace ${data.namespace}`;
};

const InstallHelmChart = ({
  namespace,
  clusterName,
  joinToken,
  pollingTimeout,
  nextStep,
  updateAgentMeta,
}: {
  namespace: string;
  clusterName: string;
  joinToken: JoinToken;
  pollingTimeout: number;
  nextStep(): void;
  updateAgentMeta(a: AgentMeta): void;
}) => {
  const ctx = useTeleport();

  const version = ctx.storeUser.state.cluster.authVersion;
  const { hostname, port } = window.document.location;
  const host = `${hostname}:${port || '443'}`;

  // Starts resource querying interval.
  const { timedOut: pollingTimedOut, result } = usePingTeleport<Kube>();

  let pollState: PollState = 'polling';
  if (pollingTimedOut) {
    pollState = 'error';
  } else if (result) {
    pollState = 'success';
  }

  function handleOnProceed() {
    updateAgentMeta({
      kube: result,
      resourceName: result.name,
    } as KubeMeta);

    nextStep();
  }

  return (
    <>
      <PollBox mt={4} p={3} borderRadius={3} pollState={pollState}>
        <Text bold>Step 3</Text>
        <Text typography="subtitle1" mb={3}>
          Run the command below on the server running your Kubernetes cluster.
          May take up to a minute for the Teleport Service to join after running
          the command.
        </Text>
        <Box mt={2} mb={1}>
          <TextSelectCopyMulti
            lines={[
              {
                text: generateCmd({
                  namespace,
                  clusterName,
                  proxyAddr: host,
                  tokenId: joinToken.id,
                  clusterVersion: version,
                  resourceId: joinToken.internalResourceId,
                }),
              },
            ]}
          />
        </Box>
        {pollState === 'polling' && (
          <TextIcon
            css={`
              white-space: pre;
            `}
          >
            <Icons.Restore fontSize={4} />
            <Timeout
              timeout={pollingTimeout}
              message="Waiting for Teleport Service  |  "
            />
          </TextIcon>
        )}
        {pollState === 'success' && (
          <TextIcon>
            <Icons.CircleCheck ml={1} color="success" />
            The Teleport Service successfully join this Teleport cluster
          </TextIcon>
        )}
        {pollState === 'error' && <TimeoutError namespace={namespace} />}
      </PollBox>
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={pollState !== 'success'}
      />
    </>
  );
};

const TimeoutError = ({ namespace }: { namespace: string }) => {
  return (
    <Box>
      <TextIcon>
        <Icons.Warning ml={1} color="danger" />
        We could not detect the Teleport Service you were trying to add
      </TextIcon>
      <Text bold mt={4}>
        Possible reasons
      </Text>
      <ul
        css={`
          margin-top: 6px;
          margin-bottom: 0;
        `}
      >
        <li>
          The command was not run on the server you were trying to add,
          regenerate command and try again.
        </li>
        <li>
          The Teleport Service could not join this Teleport cluster. Check the
          logs for errors by running <br />
          <Mark>kubectl logs -l app=teleport-agent -n {namespace}</Mark>
        </li>
      </ul>
    </Box>
  );
};

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  padding: 20px;
`;
