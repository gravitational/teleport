/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import { useQueryClient } from '@tanstack/react-query';
import { useMemo, useState } from 'react';
import { useHistory } from 'react-router';
import { styled } from 'styled-components';

import { Warning } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import {
  ButtonPrimary,
  ButtonSecondary,
  ButtonWarning,
} from 'design/Button/Button';
import { Dialog } from 'design/Dialog/Dialog';
import DialogContent from 'design/Dialog/DialogContent';
import DialogHeader from 'design/Dialog/DialogHeader';
import DialogTitle from 'design/Dialog/DialogTitle';
import Flex from 'design/Flex/Flex';
import Link from 'design/Link/Link';
import { H2, P2 } from 'design/Text/Text';
import { FieldSelectCreatableAsync } from 'shared/components/FieldSelect/FieldSelectCreatable';
import { Rule } from 'shared/components/Validation/rules';
import Validator, { Validation } from 'shared/components/Validation/Validation';

import cfg from 'teleport/config';
import { Kube } from 'teleport/services/kube/types';
import {
  IntegrationEnrollField,
  IntegrationEnrollStatusCode,
  IntegrationEnrollStep,
} from 'teleport/services/userEvent';
import useTeleport from 'teleport/useTeleport';

import { FlowStepProps } from '../Shared/GuidedFlow';
import { makeKubernetesAccessChecker } from '../Shared/kubernetes';
import { useTracking } from '../Shared/useTracking';
import { CodePanel } from './CodePanel';
import { useGitHubK8sFlow } from './useGitHubK8sFlow';

export function Finish(props: FlowStepProps) {
  const { prevStep } = props;

  const { dispatch, state } = useGitHubK8sFlow();

  const [showCloseCheck, setShowCloseCheck] = useState(false);
  const [kubernetesClusters, setKubernetesClusters] = useState<Kube[]>([]);

  const history = useHistory();
  const tracking = useTracking();

  const ctx = useTeleport();
  const flags = ctx.getFeatureFlags();
  const hasKubernetesListPermission = flags.kubernetes;

  const queryClient = useQueryClient();

  const fetchClusters = async (search: string) => {
    if (!hasKubernetesListPermission) {
      return [];
    }

    // Use an imperative query instead of `useQuery` (reactive). Results need to
    // be returned from this function.
    const result = await queryClient.fetchQuery({
      queryKey: [
        'list',
        'unified_resources',
        cfg.proxyCluster,
        ['kube_cluster'],
        search,
      ],
      queryFn: ({ signal }) =>
        ctx.resourceService.fetchUnifiedResources(
          cfg.proxyCluster,
          {
            kinds: ['kube_cluster'],
            limit: 32,
            search,
          },
          signal
        ),
      staleTime: 30_000,
    });

    const clusters = result.agents.filter(
      (resource): resource is Kube => resource.kind === 'kube_cluster'
    );

    setKubernetesClusters(clusters);

    const options = clusters.map(cluster => ({
      label: cluster.name,
      value: cluster.name,
    }));

    return options ?? [];
  };

  const handleClose = (validator: Validator) => {
    if (!validator.validate()) {
      tracking.error(
        IntegrationEnrollStep.MWIGHAK8SSetupWorkflow,
        'validation error'
      );
      return;
    }

    setShowCloseCheck(true);
  };

  const handleConfirmClose = () => {
    tracking.step(
      IntegrationEnrollStep.MWIGHAK8SSetupWorkflow,
      IntegrationEnrollStatusCode.Success
    );

    history.replace(cfg.getBotsRoute());
  };

  const accessChecker = useMemo(
    () => makeKubernetesAccessChecker(state.kubernetesLabels),
    [state.kubernetesLabels]
  );

  const selectedCluster = kubernetesClusters.find(
    c => c.name === state.kubernetesCluster
  );
  const showSelectedClusterAccessWarning = selectedCluster
    ? !accessChecker.check(selectedCluster.labels)
    : false;

  return (
    <Container>
      <CodeContainer>
        <CodePanel
          trackingStep={IntegrationEnrollStep.MWIGHAK8SSetupWorkflow}
          inProgress={!state.kubernetesCluster}
        />
      </CodeContainer>

      <Validation>
        {({ validator }) => (
          <Box flex={4}>
            <H2 mb={3} mt={3}>
              Set Up Workflow
            </H2>

            <FieldSelectCreatableAsync
              label="Select a cluster to access"
              mt={2}
              isClearable
              isSearchable
              defaultOptions
              loadOptions={fetchClusters}
              rule={kubernetesClusterRule}
              value={
                state.kubernetesCluster
                  ? {
                      label: state.kubernetesCluster,
                      value: state.kubernetesCluster,
                    }
                  : null
              }
              onChange={option => {
                dispatch({
                  type: 'kubernetes-cluster-changed',
                  value: option?.value ?? '',
                });
                tracking.field(
                  IntegrationEnrollStep.MWIGHAK8SSetupWorkflow,
                  IntegrationEnrollField.MWIGHAK8SKubernetesClusterName
                );
              }}
              noOptionsMessage={() => {
                return 'Enter a cluster name manually';
              }}
              formatCreateLabel={input => `Use cluster "${input}"`}
            />

            {showSelectedClusterAccessWarning && (
              <Warning
                alignItems="flex-start"
                mt={4}
                details={
                  <>
                    Based on the labels configured, the workflow will not have
                    access to the selected cluster. Amend the labels, or select
                    another cluster.
                  </>
                }
              >
                Cluster is inaccessible
              </Warning>
            )}

            <P2>
              <strong>To complete the setup</strong>;
            </P2>
            <ol>
              <li>
                Use the Infrastructure as Code templates to create resources
              </li>
              <li>Copy the workflow template and add it to your repository</li>
              <li>Run the workflow</li>
            </ol>

            <P2>
              See the{' '}
              <Link
                target="_blank"
                href={IAC_LINK}
                onClick={() => {
                  tracking.link(
                    IntegrationEnrollStep.MWIGHAK8SSetupWorkflow,
                    IAC_LINK
                  );
                }}
              >
                Infrastructure as Code
              </Link>{' '}
              docs for information about setting up and using IaC with Teleport.
            </P2>

            <P2>
              See the{' '}
              <Link
                target="_blank"
                href={TBOT_GHA_LINK}
                onClick={() => {
                  tracking.link(
                    IntegrationEnrollStep.MWIGHAK8SSetupWorkflow,
                    TBOT_GHA_LINK
                  );
                }}
              >
                Deploying tbot on GitHub Actions
              </Link>{' '}
              docs for information about running tbot in a GitHub Actions
              workflow.
            </P2>

            <Flex gap={2} pt={5}>
              <ButtonPrimary onClick={() => handleClose(validator)}>
                Close
              </ButtonPrimary>
              <ButtonSecondary onClick={prevStep}>Back</ButtonSecondary>
            </Flex>
          </Box>
        )}
      </Validation>

      <Dialog open={showCloseCheck} onClose={() => setShowCloseCheck(false)}>
        <DialogHeader mb={4}>
          <DialogTitle>
            Are you sure you would like to complete the guide?
          </DialogTitle>
        </DialogHeader>
        <DialogContent mb={3} maxWidth={480}>
          <P2>
            Once the guide is completed, you will not longer have access to the
            Infrastructure as Code and GitHub workflow templates.
          </P2>
        </DialogContent>
        <Flex gap={3}>
          <ButtonWarning block size="large" onClick={handleConfirmClose}>
            Confirm
          </ButtonWarning>
          <ButtonSecondary
            block
            size="large"
            autoFocus
            onClick={() => setShowCloseCheck(false)}
          >
            Cancel
          </ButtonSecondary>
        </Flex>
      </Dialog>
    </Container>
  );
}

const Container = styled(Flex)`
  flex: 1;
  overflow: auto;
  gap: ${({ theme }) => theme.space[5]}px;
`;

const CodeContainer = styled(Flex)`
  flex: 6;
  flex-direction: column;
  overflow: auto;
`;

const IAC_LINK =
  'https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/';
const TBOT_GHA_LINK =
  'https://goteleport.com/docs/machine-workload-identity/deployment/github-actions/';

const kubernetesClusterRule: Rule<{ label: string; value: string } | null> =
  value => () => {
    const valid = Boolean(value?.value);
    return {
      valid,
      message: valid ? '' : 'A Kubernetes cluster is required',
    };
  };
