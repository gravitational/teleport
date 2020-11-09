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
import session from 'teleport/services/session';
import { ButtonSecondary, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

export default function RequestDenied({ reason }: Props) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Access Request Denied</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {reason && <Alert kind="danger" children={reason} />}
        <Text mb={3}>
          Your request has been denied, please contact your administrator for
          more information.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={() => session.logout()}>
          Logout
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  reason: string;
};
