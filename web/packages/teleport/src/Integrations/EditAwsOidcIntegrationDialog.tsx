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

import { useState } from 'react';

import {
  Alert,
  Box,
  ButtonBorder,
  ButtonPrimary,
  ButtonSecondary,
  Link,
  Text,
} from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import FieldInput from 'shared/components/FieldInput';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredRoleArn } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import cfg from 'teleport/config';
import {
  AwsOidcPolicyPreset,
  IntegrationAwsOidc,
  IntegrationKind,
} from 'teleport/services/integrations';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';

import { EditableIntegrationFields } from './Operations/useIntegrationOperation';

type Props = {
  close(): void;
  edit(req: EditableIntegrationFields): Promise<void>;
  integration: IntegrationAwsOidc;
};

export function EditAwsOidcIntegrationDialog(props: Props) {
  const { close, edit, integration } = props;
  const [updateAttempt, runUpdate] = useAsync(async () => {
    await edit({ kind: IntegrationKind.AwsOidc, roleArn });
  });

  const [roleArn, setRoleArn] = useState(integration.spec.roleArn);
  const [scriptUrl, setScriptUrl] = useState('');
  const [confirmed, setConfirmed] = useState(false);

  async function handleEdit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    await runUpdate();
  }

  function generateAwsOidcConfigIdpScript(
    validator: Validator,
    policyPreset: AwsOidcPolicyPreset
  ) {
    if (!validator.validate()) {
      return;
    }

    validator.reset();

    const { arnResourceName } = splitAwsIamArn(
      roleArn || props.integration.spec.roleArn
    );
    const newScriptUrl = cfg.getAwsOidcConfigureIdpScriptUrl({
      integrationName: integration.name,
      roleName: arnResourceName,
      policyPreset,
    });

    setScriptUrl(newScriptUrl);
  }

  const isProcessing = updateAttempt.status === 'processing';
  const showGenerateCommand = integration.spec.roleArn !== roleArn;

  const changeDetected = integration.spec.roleArn !== roleArn;

  function actionButtons(validator: Validator) {
    if (!scriptUrl) {
      return (
        <ButtonBorder
          mr="3"
          onClick={() =>
            generateAwsOidcConfigIdpScript(
              validator,
              AwsOidcPolicyPreset.Unspecified
            )
          }
          disabled={!roleArn || !showGenerateCommand}
        >
          Reconfigure
        </ButtonBorder>
      );
    }

    return (
      <>
        <ButtonPrimary
          mr="3"
          disabled={
            isProcessing ||
            (showGenerateCommand && !confirmed) ||
            !changeDetected
          }
          onClick={() => handleEdit(validator)}
        >
          Save
        </ButtonPrimary>
        <ButtonSecondary
          mr="3"
          onClick={() => setScriptUrl('')}
          disabled={confirmed}
        >
          Edit
        </ButtonSecondary>
      </>
    );
  }

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          onClose={close}
          open={true}
          dialogCss={() => ({
            minHeight: '324px',
            maxWidth: '650px',
            width: '100%',
          })}
        >
          <DialogHeader>
            <DialogTitle>Edit Role ARN: {integration.name}</DialogTitle>
          </DialogHeader>
          <DialogContent width="650px" m={0}>
            {updateAttempt.status === 'error' && (
              <Alert children={updateAttempt.statusText} />
            )}
            <FieldInput
              autoFocus
              label="Role ARN"
              rule={requiredRoleArn}
              value={roleArn}
              onChange={e => setRoleArn(e.target.value)}
              placeholder="arn:aws:iam::<ACCOUNT_ID>:role/<ROLE_NAME>"
              helperText="Role ARN can be found in the format: arn:aws:iam::<ACCOUNT_ID>:role/<ROLE_NAME>"
              disabled={!!scriptUrl}
            />
            {scriptUrl && (
              <Box mb={2} data-testid="scriptbox">
                <Text mb={2}>
                  Open{' '}
                  <Link
                    href="https://console.aws.amazon.com/cloudshell/home"
                    target="_blank"
                  >
                    AWS CloudShell
                  </Link>{' '}
                  and copy and paste the command that configures the permissions
                  for you:
                </Text>
                <Box>
                  <TextSelectCopyMulti
                    lines={[
                      {
                        text: `bash -c "$(curl '${scriptUrl}')"`,
                      },
                    ]}
                  />
                </Box>
              </Box>
            )}
          </DialogContent>
          <DialogFooter>
            {showGenerateCommand && scriptUrl && (
              <FieldCheckbox
                label="I ran the command"
                name="checkbox"
                checked={confirmed}
                onChange={e => {
                  setConfirmed(e.target.checked);
                }}
                disabled={isProcessing}
              />
            )}
            {actionButtons(validator)}
            <ButtonSecondary disabled={isProcessing} onClick={close}>
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}
    </Validation>
  );
}
