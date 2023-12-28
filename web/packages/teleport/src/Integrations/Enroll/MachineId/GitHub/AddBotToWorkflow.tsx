import React from 'react';

import Box from 'design/Box';
import Text from 'design/Text';
import TextEditor from 'shared/components/TextEditor';
import Flex from 'design/Flex';

import useTeleport from 'teleport/useTeleport';

import { FlowButtons } from '../shared/FlowButtons';

import { FlowStepProps } from '../shared/GuidedFlow';

import { useGitHubFlow } from './useGitHubFlow';

export function AddBotToWorkflow({ prevStep, nextStep }: FlowStepProps) {
  const { tokenName } = useGitHubFlow();
  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  const yaml = getWorkflowExampleYaml(
    cluster.authVersion,
    cluster.publicURL,
    tokenName
  );

  return (
    <Box mb="0">
      <Text bold fontSize={4} mb="3">
        Step 3: Connect Your Machine User in a GitHub Actions Workflow
      </Text>
      <Text fontSize={3} mb="3">
        Teleport has created a role, a machine user, and a join token. Below is
        an example GitHub Actions workflow doc to help you get started. You can
        find this again from the machine user’s options dropdown
      </Text>
      <Flex flex="1" height="630px" mb="3">
        <TextEditor
          readOnly={true}
          bg="levels.deep"
          data={[{ content: yaml, type: 'yaml' }]}
          onChange={console.log}
        />
      </Flex>
      <FlowButtons isLast={true} nextStep={nextStep} prevStep={prevStep} />
    </Box>
  );
}

function getWorkflowExampleYaml(
  version: string,
  proxyAddr: string,
  tokenName: string
): string {
  return `on:
push:
  branches:
  - main
jobs:
demo:
  permissions:
    # The "id-token: write" permission is required or Machine ID will not be
    # able to authenticate with the cluster.
    id-token: write
    contents: read
  # if you added a workflow name in the previous step, make sure you use the same value here
  name: machine-id-example
  runs-on: ubuntu-latest
  steps:
  - name: Checkout repository
    uses: actions/checkout@v3
  - name: Fetch Teleport binaries
    uses: teleport-actions/setup@v1
    with:
      version: ${version}
  # server access example
  - name: Fetch credentials using Machine ID
    id: auth
    uses: teleport-actions/auth@v2
    with:
      proxy: ${proxyAddr}
      token: ${tokenName}
      # Enable the submission of anonymous usage telemetry. This
      # helps us shape the future development of \`tbot\`. You can disable this
      # by omitting this.
      anonymous-telemetry: 1
  - name: List nodes (tsh)
    # Enters a command from the cluster, in this case "tsh ls" using Machine
    # ID credentials to list remote SSH nodes.
    run: tsh ls
  # kubernetes access example
  - name: Fetch kubectl
    uses: azure/setup-kubectl@v3
  - name: Fetch credentials using Machine ID
    uses: teleport-actions/auth-k8s@v2
    with:
      proxy: ${proxyAddr}
      token: ${tokenName}
      # Use the name of your Kubernetes cluster
      kubernetes-cluster: my-kubernetes-cluster
      anonymous-telemetry: 1
  - name: List pods
    run: kubectl get pods -A`;
}
