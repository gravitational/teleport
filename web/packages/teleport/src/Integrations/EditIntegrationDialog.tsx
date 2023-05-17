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
