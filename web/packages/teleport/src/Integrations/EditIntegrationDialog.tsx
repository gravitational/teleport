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

import React, { useState } from 'react';
import { ButtonSecondary, ButtonPrimary, Alert, Text } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredRoleArn } from 'shared/components/Validation/rules';

import { Integration } from 'teleport/services/integrations';

import { EditableIntegrationFields } from './Operations/useIntegrationOperation';

type Props = {
  close(): void;
  edit(req: EditableIntegrationFields): Promise<void>;
  integration: Integration;
};

export function EditIntegrationDialog(props: Props) {
  const { close, edit, integration } = props;
  const { attempt, run } = useAttempt();

  const [roleArn, setRoleArn] = useState(integration.spec.roleArn);

  const isProcessing = attempt.status === 'processing';

  function handleEdit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() => edit({ roleArn }));
  }

  return (
    <Validation>
      {({ validator }) => (
        <Dialog disableEscapeKeyDown={false} onClose={close} open={true}>
          <DialogHeader>
            <DialogTitle>Edit Integration</DialogTitle>
          </DialogHeader>
          <DialogContent width="450px">
            {attempt.status === 'failed' && (
              <Alert children={attempt.statusText} />
            )}
            <FieldInput
              label="Integration Name"
              value={integration.name}
              readonly={true}
            />
            <FieldInput
              autoFocus
              label="Role ARN"
              rule={requiredRoleArn}
              value={roleArn}
              onChange={e => setRoleArn(e.target.value)}
              toolTipContent={
                <Text>
                  Role ARN can be found in the format: <br />
                  {`arn:aws:iam::<ACCOUNT_ID>:role/<ROLE_NAME>`}
                </Text>
              }
            />
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={isProcessing || roleArn === integration.spec.roleArn}
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
