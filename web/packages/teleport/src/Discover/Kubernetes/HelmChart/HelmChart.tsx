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

import React, { Suspense, useState } from 'react';
import styled from 'styled-components';
import { Box, ButtonSecondary, Link, Text } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { CatchError } from 'teleport/components/CatchError';
import {
  clearCachedJoinTokenResult,
  useJoinTokenSuspender,
} from 'teleport/Discover/Shared/useJoinTokenSuspender';
import useTeleport from 'teleport/useTeleport';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';

import {
  HintBox,
  SuccessBox,
  WaitingInfo,
} from 'teleport/Discover/Shared/HintBox';

import { CommandBox } from 'teleport/Discover/Shared/CommandBox';

import {
  ActionButtons,
  HeaderSubtitle,
  Mark,
  ResourceKind,
  TextIcon,
  useShowHint,
  Header,
} from '../../Shared';

import type { AgentStepProps } from '../../types';
import type { JoinToken } from 'teleport/services/joinToken';
import type { AgentMeta, KubeMeta } from 'teleport/Discover/useDiscover';
import type { Kube } from 'teleport/services/kube';

export default function Container(props: AgentStepProps) {
  const [namespace, setNamespace] = useState('');
  const [clusterName, setClusterName] = useState('');
  const [showHelmChart, setShowHelmChart] = useState(false);

  return (
    // This outer CatchError and Suspense handles
    // join token api fetch error and loading states.
    <CatchError
      onRetry={() => clearCachedJoinTokenResult(ResourceKind.Kubernetes)}
      fallbackFn={fallbackProps => (
        <Box>
          <Heading />
          <StepOne />
          <StepTwo
            onEdit={() => setShowHelmChart(false)}
            error={fallbackProps.error}
            namespace={namespace}
            setNamespace={setNamespace}
            clusterName={clusterName}
            setClusterName={setClusterName}
          />
          <ActionButtons
            onProceed={() => null}
            disableProceed={true}
            onPrev={props.prevStep}
          />
        </Box>
      )}
    >
      <Suspense
        fallback={
          <Box>
            <Heading />
            <StepOne />
            <StepTwo
              onEdit={() => setShowHelmChart(false)}
              namespace={namespace}
              setNamespace={setNamespace}
              clusterName={clusterName}
              setClusterName={setClusterName}
            />
            <ActionButtons
              onProceed={() => null}
              disableProceed={true}
              onPrev={props.prevStep}
            />
          </Box>
        }
      >
        {!showHelmChart && (
          <Box>
            <Heading />
            <StepOne />
            <StepTwo
              onEdit={() => setShowHelmChart(false)}
              generateScript={() => setShowHelmChart(true)}
              namespace={namespace}
              setNamespace={setNamespace}
              clusterName={clusterName}
              setClusterName={setClusterName}
            />
            <ActionButtons
              onProceed={() => null}
              disableProceed={true}
              onPrev={props.prevStep}
            />
          </Box>
        )}
        {showHelmChart && (
          <HelmChart
            {...props}
            onEdit={() => setShowHelmChart(false)}
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
    onEdit: () => void;
    namespace: string;
    setNamespace(n: string): void;
    clusterName: string;
    setClusterName(c: string): void;
  }
) {
  const { joinToken, reloadJoinToken } = useJoinTokenSuspender(
    ResourceKind.Kubernetes
  );

  return (
    <Box>
      <Heading />
      <StepOne />
      <StepTwo
        disabled={true}
        onEdit={() => props.onEdit()}
        generateScript={reloadJoinToken}
        namespace={props.namespace}
        setNamespace={props.setNamespace}
        clusterName={props.clusterName}
        setClusterName={props.setClusterName}
      />
      <InstallHelmChart
        prevStep={props.prevStep}
        namespace={props.namespace}
        clusterName={props.clusterName}
        joinToken={joinToken}
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
  error,
  generateScript,
  disabled,
  onEdit,
}: {
  error?: Error;
  generateScript?(): void;
  namespace: string;
  setNamespace(n: string): void;
  clusterName: string;
  setClusterName(c: string): void;
  disabled?: boolean;
  onEdit: () => void;
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
                disabled={disabled}
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
                disabled={disabled}
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
            {disabled ? (
              <ButtonSecondary
                width="200px"
                type="submit"
                onClick={() => onEdit()}
              >
                Edit
              </ButtonSecondary>
            ) : (
              <ButtonSecondary
                width="200px"
                type="submit"
                onClick={() => handleSubmit(validator)}
              >
                Next
              </ButtonSecondary>
            )}
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
  isEnterprise: boolean;
  isCloud: boolean;
  automaticUpgradesEnabled: boolean;
}) => {
  let extraYAMLConfig = '';

  if (data.isEnterprise) {
    extraYAMLConfig += 'enterprise: true\n';
  }

  if (data.isCloud && data.automaticUpgradesEnabled) {
    extraYAMLConfig += 'updater:\n';
    extraYAMLConfig += '    enabled: true\n';
    extraYAMLConfig += '    releaseChannel: "stable/cloud"\n';
    extraYAMLConfig += 'highAvailability:\n';
    extraYAMLConfig += '    replicaCount: 2\n';
    extraYAMLConfig += '    podDisruptionBudget:\n';
    extraYAMLConfig += '        enabled: true\n';
    extraYAMLConfig += '        minAvailable: 1\n';
  }

  return `cat << EOF > prod-cluster-values.yaml
roles: kube
authToken: ${data.tokenId}
proxyAddr: ${data.proxyAddr}
kubeClusterName: ${data.clusterName}
labels:
    teleport.internal/resource-id: ${data.resourceId}
${extraYAMLConfig}EOF
 
helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --version ${data.clusterVersion} --create-namespace --namespace ${data.namespace}`;
};

const InstallHelmChart = ({
  namespace,
  clusterName,
  joinToken,
  nextStep,
  prevStep,
  updateAgentMeta,
}: {
  namespace: string;
  clusterName: string;
  joinToken: JoinToken;
  nextStep(): void;
  prevStep(): void;
  updateAgentMeta(a: AgentMeta): void;
}) => {
  const ctx = useTeleport();

  const version = ctx.storeUser.state.cluster.authVersion;
  const { hostname, port } = window.document.location;
  const host = `${hostname}:${port || '443'}`;

  // Starts resource querying interval.
  const { result, active } = usePingTeleport<Kube>(joinToken);

  const showHint = useShowHint(active);

  let hint;
  if (showHint && !result) {
    hint = (
      <HintBox header="We're still looking for your server">
        <Text mb={3}>
          There are a couple of possible reasons for why we haven't been able to
          detect your Kubernetes cluster.
        </Text>

        <Text mb={1}>
          - The command was not run on the server you were trying to add.
        </Text>

        <Text mb={3}>
          - The Teleport Service could not join this Teleport cluster. Check the
          logs for errors by running
          <Mark>kubectl logs -l app=teleport-agent -n {namespace}</Mark>
        </Text>

        <Text>
          We'll continue to look for your Kubernetes cluster whilst you diagnose
          the issue.
        </Text>
      </HintBox>
    );
  } else if (result) {
    hint = (
      <SuccessBox>
        Successfully detected your new Kubernetes cluster.
      </SuccessBox>
    );
  } else {
    hint = (
      <WaitingInfo>
        <TextIcon
          css={`
            white-space: pre;
          `}
        >
          <Icons.Restore fontSize={4} />
        </TextIcon>
        After running the command above, we'll automatically detect your new
        Kubernetes cluster.
      </WaitingInfo>
    );
  }

  function handleOnProceed() {
    updateAgentMeta({
      kube: result,
      resourceName: result.name,
    } as KubeMeta);

    nextStep();
  }

  const command = generateCmd({
    namespace,
    clusterName,
    proxyAddr: host,
    tokenId: joinToken.id,
    clusterVersion: version,
    resourceId: joinToken.internalResourceId,
    isEnterprise: ctx.isEnterprise,
    isCloud: ctx.isCloud,
    automaticUpgradesEnabled: ctx.automaticUpgradesEnabled,
  });

  return (
    <>
      <CommandBox
        header={
          <>
            <Text bold>Step 3</Text>
            <Text typography="subtitle1" mb={3}>
              Run the command below on the server running your Kubernetes
              cluster. It may take up to a minute for the Teleport Service to
              join after running the command.
            </Text>
          </>
        }
      >
        <TextSelectCopyMulti lines={[{ text: command }]} />
      </CommandBox>
      {hint}
      <ActionButtons
        onProceed={handleOnProceed}
        disableProceed={!result}
        onPrev={prevStep}
      />
    </>
  );
};

const StyledBox = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;
