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

export type CreateDatabaseDialogProps = {
  pollTimeout: number;
  attempt: Attempt;
  retry(): void;
  close(): void;
  next(): void;
  dbName: string;
};

export function CreateDatabaseDialog({
  pollTimeout,
  attempt,
  retry,
  close,
  next,
  dbName,
}: CreateDatabaseDialogProps) {
  let content: JSX.Element;
  if (attempt.status === 'failed') {
    content = (
      <>
        <Text mb={5}>
          <Icons.Warning ml={1} mr={2} color="error.main" />
          {attempt.statusText}
        </Text>
        <Flex>
          <ButtonPrimary mr={3} width="50%" onClick={retry}>
            Retry
          </ButtonPrimary>
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
          <Icons.Restore fontSize={4} />
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
          <Icons.Check ml={1} mr={2} color="success" />
          Database "{dbName}" successfully registered
        </Text>
        <ButtonPrimary width="100%" onClick={next}>
          Next
        </ButtonPrimary>
      </>
    );
  }

  return (
    <Dialog disableEscapeKeyDown={false} open={true}>
      <DialogContent
        width="460px"
        alignItems="center"
        mb={0}
        textAlign="center"
      >
        <Text bold caps mb={4}>
          Database Register
        </Text>
        {content}
      </DialogContent>
    </Dialog>
  );
}
