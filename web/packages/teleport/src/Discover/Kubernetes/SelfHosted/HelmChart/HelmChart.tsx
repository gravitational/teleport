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

import { Suspense, useEffect, useState } from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonSecondary,
  Flex,
  H3,
  Link,
  Mark,
  Subtitle3,
  Text,
} from 'design';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import { CatchError } from 'teleport/components/CatchError';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { CommandBox } from 'teleport/Discover/Shared/CommandBox';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { ResourceLabelTooltip } from 'teleport/Discover/Shared/ResourceLabelTooltip';
import {
  clearCachedJoinTokenResult,
  useJoinTokenSuspender,
} from 'teleport/Discover/Shared/useJoinTokenSuspender';
import type { AgentMeta, KubeMeta } from 'teleport/Discover/useDiscover';
import { ResourceLabel } from 'teleport/services/agents';
import type { JoinRole, JoinToken } from 'teleport/services/joinToken';
import type { Kube } from 'teleport/services/kube';
import useTeleport from 'teleport/useTeleport';

import {
  ActionButtons,
  Header,
  HeaderSubtitle,
  LabelsCreater,
  ResourceKind,
  TextIcon,
  useShowHint,
} from '../../../Shared';
import type { AgentStepProps } from '../../../types';

export default function Container(props: AgentStepProps) {
  const [namespace, setNamespace] = useState('');
  const [clusterName, setClusterName] = useState('');
  const [showHelmChart, setShowHelmChart] = useState(false);
  const [labels, setLabels] = useState<ResourceLabel[]>([]);

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
            labels={labels}
            onChangeLabels={setLabels}
            generateScript={fallbackProps.retry}
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
              labels={labels}
              onChangeLabels={setLabels}
              processing={true}
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
              labels={labels}
              onChangeLabels={setLabels}
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
            labels={labels}
            onChangeLabels={setLabels}
          />
        )}
      </Suspense>
    </CatchError>
  );
}

const resourceKinds = [
  ResourceKind.Kubernetes,
  ResourceKind.Application,
  ResourceKind.Discovery,
];

export function HelmChart(
  props: AgentStepProps & {
    onEdit: () => void;
    namespace: string;
    setNamespace(n: string): void;
    clusterName: string;
    setClusterName(c: string): void;
    labels: ResourceLabel[];
    onChangeLabels(l: ResourceLabel[]): void;
  }
) {
  const { joinToken, reloadJoinToken } = useJoinTokenSuspender({
    resourceKinds,
    suggestedLabels: props.labels,
  });

  useEffect(() => {
    return () => clearCachedJoinTokenResult(resourceKinds);
  });

  return (
    <Box>
      <Heading />
      <StepOne />
      <StepTwo
        showHelmChart={true}
        onEdit={() => props.onEdit()}
        generateScript={reloadJoinToken}
        namespace={props.namespace}
        setNamespace={props.setNamespace}
        clusterName={props.clusterName}
        setClusterName={props.setClusterName}
        labels={props.labels}
        onChangeLabels={props.onChangeLabels}
      />
      <InstallHelmChart
        prevStep={props.prevStep}
        namespace={props.namespace}
        clusterName={props.clusterName}
        joinToken={joinToken}
        nextStep={props.nextStep}
        updateAgentMeta={props.updateAgentMeta}
        labels={props.labels}
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
          href="https://goteleport.com/docs/reference/helm-reference/teleport-kube-agent/"
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
  showHelmChart,
  onEdit,
  labels,
  onChangeLabels,
  processing,
}: {
  error?: Error;
  generateScript?(): void;
  namespace: string;
  setNamespace(n: string): void;
  clusterName: string;
  setClusterName(c: string): void;
  showHelmChart?: boolean;
  processing?: boolean;
  onEdit: () => void;
  labels: ResourceLabel[];
  onChangeLabels(l: ResourceLabel[]): void;
}) => {
  const disabled = showHelmChart || processing;

  function handleSubmit(
    inputFieldValidator: Validator,
    labelsValidator: Validator
  ) {
    if (!inputFieldValidator.validate() || !labelsValidator.validate()) {
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
        {({ validator: inputFieldValidator }) => (
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
                helperText="Name shown to Teleport users connecting to the cluster"
                value={clusterName}
                placeholder="my-cluster"
                width="100%"
                mr="3"
                onChange={e => setClusterName(e.target.value)}
              />
            </Box>
            <Flex alignItems="center" gap={1} mb={2}>
              <Subtitle3>Add Labels (Optional)</Subtitle3>
              <ResourceLabelTooltip resourceKind="kube" toolTipPosition="top" />
            </Flex>
            <Validation>
              {({ validator: labelsValidator }) => (
                <>
                  <Box mb={3}>
                    <LabelsCreater
                      labels={labels}
                      setLabels={onChangeLabels}
                      isLabelOptional={true}
                      disableBtns={showHelmChart}
                      noDuplicateKey={true}
                    />
                  </Box>
                  {showHelmChart ? (
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
                      onClick={() =>
                        handleSubmit(inputFieldValidator, labelsValidator)
                      }
                      disabled={processing}
                    >
                      Generate Command
                    </ButtonSecondary>
                  )}
                </>
              )}
            </Validation>
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
  labels,
}: {
  namespace: string;
  clusterName: string;
  joinToken: JoinToken;
  nextStep(): void;
  prevStep(): void;
  updateAgentMeta(a: AgentMeta): void;
  labels: ResourceLabel[];
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
    const details = (
      <>
        <Text mb={3}>
          There are a couple of possible reasons for why we haven&apos;t been
          able to detect your Kubernetes cluster.
        </Text>

        <ul>
          <li>
            <Text mb={1}>
              The command was not run on the server you were trying to add.
            </Text>
          </li>
          <li>
            <Text mb={3}>
              The Teleport Service could not join this Teleport cluster. Check
              the logs for errors by running
              <Mark>kubectl logs -l app=teleport-agent -n {namespace}</Mark>
            </Text>
          </li>
        </ul>

        <Text>
          We&apos;ll continue to look for your Kubernetes cluster while you
          diagnose the issue.
        </Text>
      </>
    );
    hint = (
      <Alert kind="warning" details={details} alignItems="flex-start">
        We&apos;re still looking for your Kubernetes cluster.
      </Alert>
    );
  } else if (result) {
    hint = (
      <Alert kind="success">
        Successfully detected your new Kubernetes cluster.
      </Alert>
    );
  } else {
    hint = (
      <Alert kind="neutral" icon={Icons.Restore}>
        After running the command above, we&apos;ll automatically detect your
        new Kubernetes cluster.
      </Alert>
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
    joinLabels: labels,
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
