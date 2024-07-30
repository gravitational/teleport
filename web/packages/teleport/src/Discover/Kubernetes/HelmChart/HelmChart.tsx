/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React, { Suspense, useState } from 'react';
import styled from 'styled-components';
import { Box, ButtonSecondary, Link, Text, Mark, H3, Subtitle3 } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { P } from 'design/Text/Text';

import { ResourceLabel } from 'teleport/services/agents';
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
  Header,
  HeaderSubtitle,
  ResourceKind,
  TextIcon,
  useShowHint,
} from '../../Shared';

import type { AgentStepProps } from '../../types';
import type { JoinRole, JoinToken } from 'teleport/services/joinToken';
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
      onRetry={() =>
        clearCachedJoinTokenResult([
          ResourceKind.Kubernetes,
          ResourceKind.Application,
          ResourceKind.Discovery,
        ])
      }
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
  const { joinToken, reloadJoinToken } = useJoinTokenSuspender([
    ResourceKind.Kubernetes,
    ResourceKind.Application,
    ResourceKind.Discovery,
  ]);

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
      <header>
        <H3>Step 1</H3>
        <Subtitle3 mb={3}>
          Add teleport-agent chart to your charts repository
        </Subtitle3>
      </header>
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
      <header>
        <H3>Step 2</H3>
        <Subtitle3 mb={3}>
          Generate a command to automatically configure and install the
          teleport-agent namespace
        </Subtitle3>
      </header>
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
            <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
            Encountered Error: {error.message}
          </TextIcon>
        </Box>
      )}
    </StyledBox>
  );
};

export type GenerateCmdProps = {
  namespace: string;
  clusterName: string;
  proxyAddr: string;
  tokenId: string;
  clusterVersion: string;
  resourceId: string;
  isEnterprise: boolean;
  isCloud: boolean;
  automaticUpgradesEnabled: boolean;
  automaticUpgradesTargetVersion: string;
  joinLabels?: ResourceLabel[];
  disableAppDiscovery?: boolean;
};

export function generateCmd(data: GenerateCmdProps) {
  let extraYAMLConfig = '';
  let deployVersion = data.clusterVersion;
  let roles: JoinRole[] = ['Kube', 'App', 'Discovery'];
  if (data.disableAppDiscovery) {
    roles = ['Kube'];
  }

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

    // Replace the helm version to deploy with the one coming from the AutomaticUpgrades Version URL.
    // AutomaticUpgradesTargetVersion contains a v, eg, v13.4.2.
    // However, helm chart expects no 'v', eg, 13.4.2.
    deployVersion = data.automaticUpgradesTargetVersion.replace(/^v/, '');

    // TODO(marco): remove when stable/cloud moves to v14
    // For v13 releases of the helm chart, we must remove the App role.
    // We get the following error otherwise:
    // Error: INSTALLATION FAILED: execution error at (teleport-kube-agent/templates/statefulset.yaml:26:28): at least one of 'apps' and 'appResources' is required in chart values when app role is enabled, see README
    if (deployVersion.startsWith('13.')) {
      roles = ['Kube'];
    }
  }

  const yamlRoles = roles.join(',').toLowerCase();

  // whitespace in the beginning if a string is intentional, to correctly align in yaml.
  const joinLabelsText = data.joinLabels
    ? '\n' + data.joinLabels.map(l => `    ${l.name}: ${l.value}`).join('\n')
    : '';

  return `cat << EOF > prod-cluster-values.yaml
roles: ${yamlRoles}
authToken: ${data.tokenId}
proxyAddr: ${data.proxyAddr}
kubeClusterName: ${data.clusterName}
labels:
    teleport.internal/resource-id: ${data.resourceId}${joinLabelsText}
${extraYAMLConfig}EOF
 
helm install teleport-agent teleport/teleport-kube-agent -f prod-cluster-values.yaml --version ${deployVersion} \\
--create-namespace --namespace ${data.namespace}`;
}

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
          <Icons.Restore size="medium" mr={2} />
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
    automaticUpgradesTargetVersion: ctx.automaticUpgradesTargetVersion,
  });

  return (
    <>
      <CommandBox
        header={
          <>
            <H3>Step 3</H3>
            <P mb={3}>
              Run the command below on the server running your Kubernetes
              cluster. It may take up to a minute for the Teleport Service to
              join after running the command.
            </P>
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
