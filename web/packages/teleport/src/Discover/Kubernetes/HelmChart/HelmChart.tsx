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

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { CatchError } from 'teleport/components/CatchError';
import {
  useJoinToken,
  clearCachedJoinTokenResult,
} from 'teleport/Discover/Shared/JoinTokenContext';
import useTeleport from 'teleport/useTeleport';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { CommandWithTimer } from 'teleport/Discover/Shared/CommandWithTimer';

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
import type { Poll } from 'teleport/Discover/Shared/CommandWithTimer';

export default function Container(
  props: AgentStepProps & { runJoinTokenPromise?: boolean }
) {
  const [namespace, setNamespace] = useState('');
  const [clusterName, setClusterName] = useState('');
  const [showHelmChart, setShowHelmChart] = useState(props.runJoinTokenPromise);

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
        {!showHelmChart && (
          <Box>
            <Heading />
            <StepOne />
            <StepTwo
              generateScript={() => setShowHelmChart(true)}
              namespace={namespace}
              setNamespace={setNamespace}
              clusterName={clusterName}
              setClusterName={setClusterName}
            />
            <ActionButtons onProceed={() => null} disableProceed={true} />
          </Box>
        )}
        {showHelmChart && (
          <HelmChart
            {...props}
            namespace={namespace}
            setNamespace={setNamespace}
            clusterName={clusterName}
            setClusterName={setClusterName}
          />
        )}
      </Suspense>
    </CatchError>
  );
}

export function HelmChart(
  props: AgentStepProps & {
    namespace: string;
    setNamespace(n: string): void;
    clusterName: string;
    setClusterName(c: string): void;
  }
) {
  const { joinToken, reloadJoinToken, timeout } = useJoinToken(
    ResourceKind.Kubernetes
  );

  return (
    <Box>
      <Heading />
      <StepOne />
      <StepTwo
        generateScript={reloadJoinToken}
        namespace={props.namespace}
        setNamespace={props.setNamespace}
        clusterName={props.clusterName}
        setClusterName={props.setClusterName}
        hasJoinToken={!!joinToken}
      />
      <InstallHelmChart
        namespace={props.namespace}
        clusterName={props.clusterName}
        joinToken={joinToken}
        pollingTimeout={timeout}
        nextStep={props.nextStep}
        updateAgentMeta={props.updateAgentMeta}
      />
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
  namespace,
  setNamespace,
  clusterName,
  setClusterName,
  hasJoinToken,
  error,
  onRetry,
  generateScript,
}: {
  error?: Error;
  onRetry?(): void;
  generateScript?(): void;
  namespace: string;
  setNamespace(n: string): void;
  clusterName: string;
  setClusterName(c: string): void;
  hasJoinToken?: boolean;
}) => {
  function handleSubmit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    generateScript();
  }
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
              disabled={!error && !generateScript}
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

  let poll: Poll = { state: 'polling' };
  if (pollingTimedOut) {
    poll = {
      state: 'error',
      error: {
        reasonContents: [
          <>
            The command was not run on the server you were trying to add,
            regenerate command and try again.
          </>,
          <>
            The Teleport Service could not join this Teleport cluster. Check the
            logs for errors by running <br />
            <Mark>kubectl logs -l app=teleport-agent -n {namespace}</Mark>
          </>,
        ],
      },
    };
  } else if (result) {
    poll = { state: 'success' };
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
      <CommandWithTimer
        command={generateCmd({
          namespace,
          clusterName,
          proxyAddr: host,
          tokenId: joinToken.id,
          clusterVersion: version,
          resourceId: joinToken.internalResourceId,
        })}
        poll={poll}
        pollingTimeout={pollingTimeout}
        header={
          <>
            <Text bold>Step 3</Text>
            <Text typography="subtitle1" mb={3}>
              Run the command below on the server running your Kubernetes
              cluster. May take up to a minute for the Teleport Service to join
              after running the command.
            </Text>
          </>
        }
      />
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={poll.state !== 'success'}
      />
    </>
  );
};

const StyledBox = styled(Box)`
  max-width: 800px;
  background-color: rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  padding: 20px;
`;
