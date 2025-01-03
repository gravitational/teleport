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

import React, { Suspense, useEffect, useState } from 'react';
import styled from 'styled-components';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  H3,
  Indicator,
} from 'design';
import Dialog, { DialogContent, DialogFooter } from 'design/DialogConfirmation';
import * as Icons from 'design/Icon';
import { P } from 'design/Text/Text';

import { CatchError } from 'teleport/components/CatchError';
import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';
import { ResourceKind, TextIcon } from 'teleport/Discover/Shared';
import { CommandBox } from 'teleport/Discover/Shared/CommandBox';
import {
  clearCachedJoinTokenResult,
  useJoinTokenSuspender,
} from 'teleport/Discover/Shared/useJoinTokenSuspender';
import { JoinToken } from 'teleport/services/joinToken';

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
        <H2 mb={4}>Manual EKS Cluster Enrollment</H2>
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

const resourceKinds = [
  ResourceKind.Kubernetes,
  ResourceKind.Application,
  ResourceKind.Discovery,
];

export function ManualHelmDialog({
  setJoinTokenAndGetCommand,
  cancel,
  confirmedCommands,
}: ManualHelmDialogProps) {
  const { joinToken } = useJoinTokenSuspender({
    resourceKinds,
  });
  const [command, setCommand] = useState('');

  useEffect(() => {
    if (joinToken && !command) {
      setCommand(setJoinTokenAndGetCommand(joinToken));
    }

    return () => clearCachedJoinTokenResult(resourceKinds);
  }, [joinToken, command, setJoinTokenAndGetCommand]);

  return (
    <DialogWrapper cancel={cancel} next={confirmedCommands}>
      <StyledBox mb={5}>
        <H3>Step 1</H3>
        <P mb={3}>Add teleport-agent chart to your charts repository</P>
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
            <H3>Step 2</H3>
            <P mb={3}>
              Run the command below on the server your target EKS cluster is at.
              It may take up to a minute for the Teleport Service to join after
              running the command.
            </P>
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
