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

import React from 'react';
import {
  Text,
  Flex,
  AnimatedProgressBar,
  ButtonPrimary,
  ButtonSecondary,
  H2,
  ButtonWarning,
} from 'design';
import * as Icons from 'design/Icon';
import Dialog, { DialogContent } from 'design/DialogConfirmation';

import { Timeout } from 'teleport/Discover/Shared/Timeout';
import { TextIcon } from 'teleport/Discover/Shared';

import {
  dbWithoutDbServerExistsErrorMsg as dbExistsWithoutDbServerErrorMsg,
  timeoutErrorMsg,
} from './const';

import type { Attempt } from 'shared/hooks/useAttemptNext';

export type CreateDatabaseDialogProps = {
  pollTimeout: number;
  attempt: Attempt;
  retry(): void;
  close(): void;
  next(): void;
  onOverwrite(): void;
  onTimeout(): void;
  dbName: string;
};

export function CreateDatabaseDialog({
  pollTimeout,
  attempt,
  retry,
  close,
  next,
  dbName,
  onOverwrite,
  onTimeout,
}: CreateDatabaseDialogProps) {
  /**
   * Most likely cause of timeout is when we found a matching db_service
   * but no db_server heartbeats. Most likely cause is because db_service
   * has been stopped but is not removed from teleport yet (there is some
   * minutes delay on expiry)
   */
  const skipOnTimeout =
    attempt.status === 'failed' && attempt.statusText === timeoutErrorMsg;

  let content: JSX.Element;
  if (attempt.status === 'failed' && !skipOnTimeout) {
    // Only allow overwriting if the database error
    // states that it's a existing database without a db_server.
    const canOverwriteDb = attempt.statusText.includes(
      dbExistsWithoutDbServerErrorMsg
    );

    // TODO(bl-nero): Migrate this to alert boxes.
    content = (
      <>
        <Flex mb={5} alignItems="center">
          <Icons.Warning size="large" ml={1} mr={2} color="error.main" />
          <Text>{attempt.statusText}</Text>
        </Flex>
        <Flex gap={3}>
          <ButtonPrimary width="50%" onClick={retry}>
            Retry
          </ButtonPrimary>
          {canOverwriteDb && (
            <ButtonWarning width="50%" onClick={onOverwrite}>
              Overwrite
            </ButtonWarning>
          )}
          <ButtonSecondary width="50%" onClick={close}>
            Close
          </ButtonSecondary>
        </Flex>
      </>
    );
  } else if (attempt.status === 'processing') {
    content = (
      <>
        <AnimatedProgressBar mb={1} />
        <TextIcon
          mb={3}
          css={`
            white-space: pre;
          `}
        >
          <Icons.Clock size="medium" />
          <Timeout
            timeout={pollTimeout}
            message=""
            tailMessage={' seconds left'}
          />
        </TextIcon>
        <ButtonPrimary width="100%" disabled>
          Next
        </ButtonPrimary>
      </>
    );
  } else {
    // success
    content = (
      <>
        <Text mb={5}>
          <Icons.Check size="small" ml={1} mr={2} color="success.main" />
          Database "{dbName}" successfully registered
        </Text>
        <ButtonPrimary width="100%" onClick={skipOnTimeout ? onTimeout : next}>
          Next
        </ButtonPrimary>
      </>
    );
  }

  return (
    <Dialog open={true}>
      <DialogContent
        width="460px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        <H2 mb={4}>Database Register</H2>
        {content}
      </DialogContent>
    </Dialog>
  );
}
