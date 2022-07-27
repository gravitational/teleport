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

import { Text, ButtonPrimary, ButtonSecondary, Box } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import cfg from 'teleport/config';
import history from 'teleport/services/history';

export const Header: React.FC = ({ children }) => (
  <Text mb={4} typography="h4" bold>
    {children}
  </Text>
);

export const ActionButtons = ({ onProceed }: { onProceed(): void }) => {
  const [confirmExit, setConfirmExit] = React.useState(false);
  return (
    <Box mt={4}>
      <ButtonPrimary width="165px" onClick={onProceed} mr={3}>
        Proceed
      </ButtonPrimary>
      <ButtonSecondary
        mt={3}
        width="165px"
        onClick={() => setConfirmExit(true)}
      >
        Exit
      </ButtonSecondary>
      {confirmExit && (
        <ConfirmExitDialog onClose={() => setConfirmExit(false)} />
      )}
    </Box>
  );
};

function ConfirmExitDialog({ onClose }: { onClose(): void }) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '600px' })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Exit Resource Connection</DialogTitle>
      </DialogHeader>
      <DialogContent minWidth="500px" flex="0 0 auto">
        <Text mb={2}>
          Are you sure you want to cancel the Resource connection process?
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonPrimary
          mr="3"
          onClick={() => history.push(cfg.routes.root, true)}
        >
          Yes
        </ButtonPrimary>
        <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
