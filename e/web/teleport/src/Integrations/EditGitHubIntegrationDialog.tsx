/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { useState } from 'react';

import { Alert, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { useAsync } from 'shared/hooks/useAsync';

import {
  IntegrationGitHub,
  IntegrationKind,
} from 'teleport/services/integrations';

import { EditableIntegrationFieldsE } from './useIntegrations';

export function EditGitHubIntegrationDialog(props: {
  close(): void;
  edit(req: EditableIntegrationFieldsE): Promise<void>;
  integration: IntegrationGitHub;
}) {
  const { close, edit, integration } = props;

  const [newSecret, setNewSecret] = useState('');

  const [updateAttempt, runUpdate] = useAsync(async () => {
    await edit({
      kind: IntegrationKind.GitHub,
      secret: newSecret,
    });
  });

  async function handleEdit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    await runUpdate();
  }

  const isProcessing = updateAttempt.status === 'processing';

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          onClose={close}
          open={true}
          dialogCss={() => ({
            maxWidth: '650px',
            width: '100%',
          })}
        >
          <DialogHeader>
            <DialogTitle>Edit GitHub Integration</DialogTitle>
          </DialogHeader>
          <DialogContent width="650px">
            {updateAttempt.status === 'error' && (
              <Alert>{updateAttempt.statusText}</Alert>
            )}
            <FieldInput
              label="Integration Name"
              value={integration.name}
              readonly
            />
            <FieldInput
              label="New Client Secret"
              type="password"
              value={newSecret}
              onChange={e => setNewSecret(e.target.value)}
              placeholder="12a3b45c6d123456aaa...1a2b3456"
            />
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={isProcessing || !newSecret}
              onClick={() => handleEdit(validator)}
            >
              Save
            </ButtonPrimary>
            <ButtonSecondary disabled={isProcessing} onClick={close}>
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}
    </Validation>
  );
}
