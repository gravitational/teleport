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

import Dialog, { DialogContent, DialogFooter } from 'design/DialogConfirmation';
import {
  Box,
  Flex,
  ButtonPrimary,
  ButtonSecondary,
  Text,
  Indicator,
} from 'design';

import React, { Suspense, useState, useEffect } from 'react';

import styled from 'styled-components';

import * as Icons from 'design/Icon';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { CommandBox } from 'teleport/Discover/Shared/CommandBox';

import { useJoinTokenSuspender } from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { ResourceKind, TextIcon } from 'teleport/Discover/Shared';
import { JoinToken } from 'teleport/services/joinToken';
import { CatchError } from 'teleport/components/CatchError';

type ManualHelmDialogProps = {
  setJoinTokenAndGetCommand(token: JoinToken): string;
  confirmedCommands(): void;
  cancel(): void;
};

export default function Container(props: ManualHelmDialogProps) {
  return (
    <CatchError
      fallbackFn={fallbackProps => (
        <FallbackDialog error={fallbackProps.error} cancel={props.cancel} />
      )}
    >
      <Suspense
        fallback={<FallbackDialog showSpinner={true} cancel={props.cancel} />}
      >
        <ManualHelmDialog {...props} />
      </Suspense>
    </CatchError>
  );
}

type FallbackDialogProps = {
  cancel: () => void;
  error?: Error;
  showSpinner?: boolean;
};

const DialogWrapper = ({
  children,
  cancel,
  next,
}: {
  children: React.ReactNode;
  cancel: () => void;
  next?: () => void;
}) => {
  return (
    <Dialog onClose={cancel} open={true}>
      <DialogContent width="850px" mb={0}>
        <Text bold caps mb={4}>
          Manual EKS Cluster Enrollment
        </Text>
        {children}
      </DialogContent>
      <DialogFooter alignItems="center" as={Flex} gap={4}>
        <ButtonPrimary width="50%" onClick={next} disabled={!next}>
          I ran these commands
        </ButtonPrimary>
        <ButtonSecondary width="50%" onClick={cancel}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
};

const FallbackDialog = ({
  error,
  cancel,
  showSpinner,
}: FallbackDialogProps) => {
  return (
    <DialogWrapper cancel={cancel}>
      {showSpinner && (
        <Flex mb={4} justifyContent="center">
          <Indicator delay="none" />
        </Flex>
      )}
      {error && (
        <Box mb={4}>
          <TextIcon mt={3}>
            <Icons.Warning size="medium" ml={1} mr={2} color="error.main" />
            Encountered an error: {error.message}
          </TextIcon>
        </Box>
      )}
    </DialogWrapper>
  );
};

export function ManualHelmDialog({
  setJoinTokenAndGetCommand,
  cancel,
  confirmedCommands,
}: ManualHelmDialogProps) {
  const { joinToken } = useJoinTokenSuspender([
    ResourceKind.Kubernetes,
    ResourceKind.Application,
    ResourceKind.Discovery,
  ]);
  const [command, setCommand] = useState('');

  useEffect(() => {
    if (joinToken && !command) {
      setCommand(setJoinTokenAndGetCommand(joinToken));
    }
  }, [joinToken, command, setJoinTokenAndGetCommand]);

  return (
    <DialogWrapper cancel={cancel} next={confirmedCommands}>
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
      <CommandBox
        header={
          <>
            <Text bold>Step 2</Text>
            <Text typography="subtitle1" mb={3}>
              Run the command below on the server your target EKS cluster is at.
              It may take up to a minute for the Teleport Service to join after
              running the command.
            </Text>
          </>
        }
      >
        <TextSelectCopyMulti lines={[{ text: command }]} />
      </CommandBox>
    </DialogWrapper>
  );
}

const StyledBox = styled(Box)`
  max-width: 1000px;
  background-color: ${props => props.theme.colors.spotBackground[0]};
  padding: ${props => `${props.theme.space[3]}px`};
  border-radius: ${props => `${props.theme.space[2]}px`};
`;
