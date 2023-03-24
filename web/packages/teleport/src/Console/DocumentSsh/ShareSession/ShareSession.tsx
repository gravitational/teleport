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
import { Text, ButtonSecondary } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import TextSelectCopy from 'teleport/components/TextSelectCopy';

export default function ShareSession({ closeShareSession }: Props) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={closeShareSession}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Share Session</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Text mb={2} mt={1}>
          Share this URL with the person you want to share your session with.
          This person must have access to this server to be able to join your
          session.
        </Text>
        <TextSelectCopy text={window.location.href} bash={false} />
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={closeShareSession}>Close</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  closeShareSession: () => void;
};
