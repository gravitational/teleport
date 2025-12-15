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

import type { JSX } from 'react';

import {
  AnimatedProgressBar,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Mark,
  Text,
} from 'design';
import Dialog, { DialogContent } from 'design/DialogConfirmation';
import * as Icons from 'design/Icon';
import type { Attempt } from 'shared/hooks/useAttemptNext';

export type CreatedDiscoveryConfigDialog = {
  attempt: Attempt;
  retry(): void;
  close(): void;
  next(): void;
  region: string;
  /**
   * show notification that resources might take some
   * time to appear after setup has finished.
   */
  notifyAboutDelay: boolean;
};

export function CreatedDiscoveryConfigDialog({
  attempt,
  retry,
  close,
  next,
  region,
  notifyAboutDelay,
}: CreatedDiscoveryConfigDialog) {
  let content: JSX.Element;
  if (attempt.status === 'failed') {
    content = (
      <>
        <Flex mb={5} alignItems="center">
          <Icons.Warning size="large" ml={1} mr={2} color="error.main" />
          <Text>{attempt.statusText}</Text>
        </Flex>
        <Flex gap={3} width="100%">
          <ButtonPrimary style={{ flex: 1 }} onClick={retry}>
            Retry
          </ButtonPrimary>
          <ButtonSecondary style={{ flex: 1 }} onClick={close}>
            Close
          </ButtonSecondary>
        </Flex>
      </>
    );
  } else if (attempt.status === 'processing') {
    content = (
      <>
        <AnimatedProgressBar mb={5} />
        <ButtonPrimary width="100%" disabled>
          Next
        </ButtonPrimary>
      </>
    );
  } else {
    // success
    content = (
      <>
        <Flex mb={5}>
          <Icons.Check size="small" ml={1} mr={2} color="success.main" />
          <Text>
            Discovery config successfully created.
            {notifyAboutDelay && (
              <>
                {' '}
                The discovery service can take a few minutes to finish
                auto-enrolling resources found in region <Mark>{region}</Mark>.
              </>
            )}
          </Text>
        </Flex>
        <ButtonPrimary width="100%" onClick={next}>
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
        <Text bold caps mb={4}>
          Creating Auto Discovery Config
        </Text>
        {content}
      </DialogContent>
    </Dialog>
  );
}
