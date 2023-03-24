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

import React from 'react';
import { formatDistanceStrict } from 'date-fns';

import { ButtonSecondary, Text } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import TextSelectCopy from 'teleport/components/TextSelectCopy';
import { ResetToken } from 'teleport/services/user';
import cfg from 'teleport/config';

export default function UserTokenLink({
  token,
  onClose,
  asInvite = false,
}: Props) {
  const tokenUrl = cfg.getUserResetTokenRoute(token.value, asInvite);
  const expiresText = formatDistanceStrict(Date.now(), token.expires);

  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={close}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Share Link</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {asInvite ? (
          <Text mb={4} mt={1}>
            User
            <Text bold as="span">
              {` ${token.username} `}
            </Text>
            has been created but requires a password. Share this URL with the
            user to set up a password, link is valid for {expiresText}.
          </Text>
        ) : (
          <Text mb={4} mt={1}>
            User
            <Text bold as="span">
              {` ${token.username} `}
            </Text>
            has been reset. Share this URL with the user to set up a new
            password, link is valid for {expiresText}.
          </Text>
        )}
        <TextSelectCopy text={tokenUrl} bash={false} />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={onClose}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  token: ResetToken;
  onClose(): void;
  asInvite?: boolean;
};
