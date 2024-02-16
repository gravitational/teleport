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
import { Text, Alert, ButtonSecondary } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import history from 'teleport/services/history';

export function ErrorDialog({ errMsg }: { errMsg: string }) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>An error has occurred</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Alert kind="danger" children={errMsg} />
        <Text mb={3}>Try again by refreshing the page.</Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary
          onClick={() => history.goToLogin(true /* rememberLocation */)}
        >
          Go to Login
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
