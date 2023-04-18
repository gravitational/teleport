/**
 * Copyright 2023 Gravitational, Inc.
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
import React from 'react';
import {
  Text,
  Flex,
  AnimatedProgressBar,
  ButtonPrimary,
  ButtonSecondary,
} from 'design';
import * as Icons from 'design/Icon';
import Dialog, { DialogContent } from 'design/DialogConfirmation';

import { Timeout } from 'teleport/Discover/Shared/Timeout';
import { TextIcon } from 'teleport/Discover/Shared';

import type { Attempt } from 'shared/hooks/useAttemptNext';

export type Props = {
  pollTimeout: number;
  attempt: Attempt;
  retry(): void;
  close(): void;
  next(): void;
  dbName: string;
  alteredRdsDbName?: string;
};

export function CreateDatabaseDialog({
  pollTimeout,
  attempt,
  retry,
  close,
  next,
  dbName,
  alteredRdsDbName,
}: Props) {
  let content;
  if (attempt.status === 'failed') {
    content = (
      <>
        <Text bold caps mb={3}>
          Database Register Failed
        </Text>
        <Text mb={5}>
          <Icons.Warning ml={1} mr={2} color="danger" />
          Error: {attempt.statusText}
        </Text>
        <Flex>
          <ButtonPrimary mr={2} width="50%" onClick={retry}>
            Retry
          </ButtonPrimary>
          {!alteredRdsDbName && (
            <ButtonSecondary width="50%" onClick={close}>
              Close
            </ButtonSecondary>
          )}
        </Flex>
      </>
    );
  } else if (attempt.status === 'processing') {
    content = (
      <>
        <Text bold caps mb={4}>
          Registering Database
        </Text>
        <AnimatedProgressBar />
        <TextIcon
          css={`
            white-space: pre;
          `}
        >
          <Icons.Restore fontSize={4} />
          <Timeout
            timeout={pollTimeout}
            message=""
            tailMessage={' seconds left'}
          />
        </TextIcon>
      </>
    );
  } else {
    // success
    content = (
      <>
        <Text bold caps mb={4}>
          Successfully Registered Database
        </Text>
        <Text mb={5}>
          <Icons.Check ml={1} mr={2} color="success" />
          {alteredRdsDbName ? (
            <>
              AWS RDS database "{dbName}"" has been registered as "
              {alteredRdsDbName}"
            </>
          ) : (
            <>Database "{dbName}" successfully registered</>
          )}
        </Text>
        <ButtonPrimary mr={2} width="100%" onClick={next}>
          Next
        </ButtonPrimary>
      </>
    );
  }

  return (
    <Dialog disableEscapeKeyDown={false} open={true}>
      <DialogContent
        width="400px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        {content}
      </DialogContent>
    </Dialog>
  );
}
