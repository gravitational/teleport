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
import { useEffect } from 'react';

import {
  Alert,
  AnimatedProgressBar,
  Box,
  ButtonPrimary,
  Flex,
  Mark,
  Text,
} from 'design';
import Dialog, { DialogContent } from 'design/DialogConfirmation';
import * as Icons from 'design/Icon';

import { TextIcon, useShowHint } from 'teleport/Discover/Shared';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { Kube } from 'teleport/services/kube';

type AgentWaitingDialogProps = {
  joinResourceId: string;
  status: string;
  clusterName: string;
  updateWaitingResult(cluster: Kube): void;
  cancel(): void;
  next(): void;
};

export function AgentWaitingDialog({
  joinResourceId,
  status,
  clusterName,
  updateWaitingResult,
  cancel,
  next,
}: AgentWaitingDialogProps) {
  const { result, active } = usePingTeleport<Kube>({
    internalResourceId: joinResourceId,

    // These are not used by usePingTeleport
    // todo(anton): Refactor usePingTeleport to not require full join token.
    expiry: undefined,
    safeName: '',
    isStatic: false,
    method: 'kubernetes',
    roles: [],
    content: '',
    expiryText: '',
    id: '',
    suggestedLabels: [],
  });
  useEffect(() => {
    if (result) {
      updateWaitingResult(result);
    }
  }, [result]);

  const showHint = useShowHint(active);

  function hintMessage() {
    const details = (
      <>
        <Text mb={3}>
          There are a few of possible reasons for why we haven&apos;t been able
          to detect your Kubernetes cluster.
        </Text>

        <ul>
          <li>
            <Text mb={1}>The cluster doesn&apos;t have active nodes.</Text>
          </li>
          <li>
            <Text mb={1}>
              The manual command was not run on the server you were trying to
              add.
            </Text>
          </li>
          <li>
            <Text mb={3}>
              The Teleport Service could not join this Teleport cluster. Check
              the logs for errors by running
              <br />
              <Mark>
                kubectl logs -l app=teleport-kube-agent -n teleport-agent
              </Mark>
            </Text>
          </li>
        </ul>

        <Text>
          We&apos;ll continue to look for your Kubernetes cluster while you
          diagnose the issue.
        </Text>
      </>
    );
    if (showHint && !result) {
      return (
        <Box textAlign={'left'} mb={3}>
          <Alert kind="warning" alignItems="flex-start" details={details}>
            We&apos;re still looking for your Kubernetes cluster
          </Alert>
        </Box>
      );
    }
  }

  function content() {
    if (status === 'awaitingAgent') {
      return (
        <>
          <Text bold caps mb={4}>
            EKS Cluster Enrollment
          </Text>
          <AnimatedProgressBar mb={3} />
          <TextIcon mb={3}>
            <Icons.Check size="medium" color="success.main" />
            <Text>1. Installing Teleport agent</Text>
          </TextIcon>
          <TextIcon mb={3}>
            <Icons.Clock size="medium" />
            <Text>
              2. Waiting for the Teleport agent to come online (1-5 minutes)...
            </Text>
          </TextIcon>
          {hintMessage()}
          <ButtonPrimary width="100%" onClick={cancel}>
            Cancel
          </ButtonPrimary>
        </>
      );
    } else {
      return (
        <>
          <Text bold caps mb={4}>
            EKS Cluster Enrollment
          </Text>
          <Flex mb={3}>
            <Icons.Check size="small" ml={1} mr={2} color="success.main" />
            Cluster "{clusterName}" was successfully enrolled.
          </Flex>
          <ButtonPrimary width="100%" onClick={next}>
            Next
          </ButtonPrimary>
        </>
      );
    }
  }

  return (
    <Dialog open={true}>
      <DialogContent
        width="460px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        {content()}
      </DialogContent>
    </Dialog>
  );
}
