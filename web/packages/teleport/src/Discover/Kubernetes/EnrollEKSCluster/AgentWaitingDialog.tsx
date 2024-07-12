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
import Dialog, { DialogContent } from 'design/DialogConfirmation';
import {
  AnimatedProgressBar,
  Box,
  ButtonPrimary,
  Text,
  Flex,
  Mark,
} from 'design';
import React, { useEffect } from 'react';

import * as Icons from 'design/Icon';

import { Kube } from 'teleport/services/kube';
import { usePingTeleport } from 'teleport/Discover/Shared/PingTeleportContext';
import { TextIcon, useShowHint } from 'teleport/Discover/Shared';
import { HintBox } from 'teleport/Discover/Shared/HintBox';

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
    if (showHint && !result) {
      return (
        <Box textAlign={'left'} mb={3}>
          <HintBox header="We're still looking for your server">
            <Text mb={3}>
              There are a few of possible reasons for why we haven't been able
              to detect your Kubernetes cluster.
            </Text>

            <Text mb={1}>- The cluster doesn't have active nodes.</Text>

            <Text mb={1}>
              - The manual command was not run on the server you were trying to
              add.
            </Text>

            <Text mb={3}>
              - The Teleport Service could not join this Teleport cluster. Check
              the logs for errors by running
              <br />
              <Mark>kubectl logs -l app=teleport-agent -n teleport-agent</Mark>
            </Text>

            <Text>
              We'll continue to look for your Kubernetes cluster whilst you
              diagnose the issue.
            </Text>
          </HintBox>
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
