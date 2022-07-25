/**
 * Copyright 2020 Gravitational, Inc.
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

import React, { useEffect } from 'react';

import {
  Alert,
  Text,
  Indicator,
  Box,
  ButtonLink,
  ButtonSecondary,
} from 'design';
import { DialogContent, DialogFooter } from 'design/Dialog';

import TextSelectCopy from 'teleport/components/TextSelectCopy';

import { createBashCommand, State } from './../useAddNode';

export default function Automatically(props: Props) {
  const { createJoinToken, attempt, onClose, joinToken } = props;

  useEffect(() => {
    if (!joinToken) {
      createJoinToken();
    }
  }, []);

  if (attempt.status === 'processing' || attempt.status == '') {
    return (
      <Box textAlign="center">
        <Indicator />
      </Box>
    );
  }

  if (attempt.status === 'failed') {
    return <Alert kind="danger" children={attempt.statusText} />;
  }

  return (
    <>
      <DialogContent>
        <Text>
          Use below script to add a server to your cluster. This script will
          install the Teleport agent to provide secure access to your server.
          <Text mt="3">
            The script will be valid for{' '}
            <Text bold as={'span'}>
              {joinToken.expiryText}.
            </Text>
          </Text>
        </Text>
        <TextSelectCopy text={createBashCommand(joinToken.id)} mt={2} />
        <Box>
          <ButtonLink onClick={createJoinToken}>Regenerate Script</ButtonLink>
        </Box>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </>
  );
}

type Props = {
  createJoinToken: State['createJoinToken'];
  attempt: State['attempt'];
  joinToken: State['token'];
  onClose(): void;
};
