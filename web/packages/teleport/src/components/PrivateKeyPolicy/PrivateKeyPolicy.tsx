/**
 * Copyright 2022 Gravitational, Inc.
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
import { Card, Text, Link, ButtonText, ButtonSecondary, Box } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { Danger } from 'design/Alert';

import { generateTshLoginCommand } from 'teleport/lib/util';

import { TextSelectCopyMulti } from 'teleport/components/TextSelectCopy';

import type { TshLoginCommand } from 'teleport/lib/util';

const LINK_HARDWARE_KEY_SUPPORT =
  'https://goteleport.com/docs/access-controls/guides/hardware-key-support/';

const LINK_TSH =
  'https://goteleport.com/docs/connect-your-client/tsh/#installing-tsh';

const LINK_CONNECT =
  'https://goteleport.com/docs/connect-your-client/teleport-connect/';

export const PrivateKeyLoginDisabledCard = ({
  title,
  onRecover,
}: {
  title: string;
  // onRecover only applies to Teleport Cloud,
  // and is called upon when user needs to recover
  // lost password or two-factor device.
  onRecover?: (isRecoverPassword: boolean) => void;
}) => (
  <Card bg="levels.surface" my="5" mx="auto" width="464px" px={5} pb={4}>
    <Text typography="h3" pt={4} textAlign="center" color="light">
      {title}
    </Text>
    <Danger my={5}>Web UI Login Disabled</Danger>
    <Text mb={2} typography="paragraph2">
      This Teleport Cluster requires that user{' '}
      <Link color="light" href={LINK_HARDWARE_KEY_SUPPORT} target="_blank">
        private keys
      </Link>{' '}
      be stored on hardware authentication devices. Since these keys are not
      accessible by web browsers, Web UI login has been disabled. Please use{' '}
      <Link color="light" href={LINK_CONNECT} target="_blank">
        Teleport Connect
      </Link>{' '}
      or{' '}
      <Link color="light" href={LINK_TSH} target="_blank">
        tsh
      </Link>{' '}
      to log in.
    </Text>
    {onRecover && (
      <Text typography="paragraph2" textAlign="center" mt={4}>
        <ButtonText
          onClick={() => onRecover(true)}
          style={{ padding: '0px', minHeight: 0 }}
          mr={2}
        >
          Forgot Password?
        </ButtonText>
        or{' '}
        <ButtonText
          onClick={onRecover}
          style={{ padding: '0px', minHeight: 0 }}
          ml={1}
        >
          Lost Two-Factor Device?
        </ButtonText>
      </Text>
    )}
  </Card>
);

export type PrivateKeyAccessRequest = TshLoginCommand & {
  accessRequestId: string;
};

export function PrivateKeyAccessRequestDialogue({
  onClose,
  btnText,
  ...tshProps
}: PrivateKeyAccessRequest & {
  btnText?: string;
  onClose(): void;
}) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      onClose={close}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Private Key Policy</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Text mb={4}>
          This access requires use of hardware backed{' '}
          <Link color="light" href={LINK_HARDWARE_KEY_SUPPORT} target="_blank">
            private keys
          </Link>{' '}
          which are not supported in the web. Please use{' '}
          <Link color="light" href={LINK_TSH} target="_blank">
            tsh
          </Link>{' '}
          to login with the approved request ID or use{' '}
          <Link color="light" href={LINK_CONNECT} target="_blank">
            Teleport Connect
          </Link>
          .
        </Text>
        <Box mb={2}>
          <Text bold>tsh login command with the requested access</Text>
          <TextSelectCopyMulti
            lines={[{ text: generateTshLoginCommand(tshProps) }]}
          />
        </Box>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>{btnText || 'Okay'}</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
