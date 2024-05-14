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
import styled from 'styled-components';
import {
  ButtonSecondary,
  ButtonPrimary,
  ButtonBorder,
  Alert,
  Text,
  Box,
  Link,
} from 'design';
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
import { CheckboxInput } from 'design/Checkbox';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import { Integration } from 'teleport/services/integrations';
import cfg from 'teleport/config';
import { splitAwsIamArn } from 'teleport/services/integrations/aws';

import { EditableIntegrationFields } from './Operations/useIntegrationOperation';
import { S3BucketConfiguration } from './Enroll/AwsOidc/S3BucketConfiguration';
import { S3BucketWarningBanner } from './Enroll/AwsOidc/S3BucketWarningBanner';

type Props = {
  close(): void;
  edit(req: EditableIntegrationFields): Promise<void>;
  integration: Integration;
};

export function EditAwsOidcIntegrationDialog(props: Props) {
  const { close, edit, integration } = props;
  const { attempt, run } = useAttempt();

  const [showS3BucketWarning, setShowS3BucketWarning] = useState(false);
  const [roleArn, setRoleArn] = useState(integration.spec.roleArn);
  const [s3Bucket, setS3Bucket] = useState(
    () => integration.spec.issuerS3Bucket
  );
  const [s3Prefix, setS3Prefix] = useState(
    () => integration.spec.issuerS3Prefix
  );

  const [scriptUrl, setScriptUrl] = useState('');
  const [confirmed, setConfirmed] = useState(false);

  function handleEdit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    run(() => edit({ roleArn, s3Bucket, s3Prefix }));
  }

  function generateAwsOidcConfigIdpScript(validator: Validator) {
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
      s3Bucket: s3Bucket,
      s3Prefix: s3Prefix,
    });

    setScriptUrl(newScriptUrl);
  }

  const isProcessing = attempt.status === 'processing';
  const requiresS3BucketWarning = !s3Bucket && !s3Prefix;
  const showGenerateCommand =
    integration.spec.issuerS3Bucket !== s3Bucket ||
    integration.spec.issuerS3Prefix !== s3Prefix ||
    integration.spec.roleArn !== roleArn;

  const changeDetected =
    integration.spec.issuerS3Bucket !== s3Bucket ||
    integration.spec.issuerS3Prefix !== s3Prefix ||
    integration.spec.roleArn !== roleArn;

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          disableEscapeKeyDown={false}
          onClose={close}
          open={true}
          dialogCss={() => ({
            maxWidth: '650px',
            width: '100%',
          })}
        >
          <DialogHeader>
            <DialogTitle>Edit Integration</DialogTitle>
          </DialogHeader>
          <DialogContent width="650px">
            {attempt.status === 'failed' && (
              <Alert children={attempt.statusText} />
            )}
            <FieldInput
              label="Integration Name"
              value={integration.name}
              readonly={true}
            />
            <EditableBox px={3} pt={2}>
              <FieldInput
                autoFocus
                label="Role ARN"
                rule={requiredRoleArn}
                value={roleArn}
                onChange={e => setRoleArn(e.target.value)}
                placeholder="arn:aws:iam::<ACCOUNT_ID>:role/<ROLE_NAME>"
                toolTipContent={
                  <Text>
                    Role ARN can be found in the format: <br />
                    {`arn:aws:iam::<ACCOUNT_ID>:role/<ROLE_NAME>`}
                  </Text>
                }
                disabled={scriptUrl}
              />
              <S3BucketConfiguration
                s3Bucket={s3Bucket}
                setS3Bucket={setS3Bucket}
                s3Prefix={s3Prefix}
                setS3Prefix={setS3Prefix}
                disabled={!!scriptUrl}
              />
              {scriptUrl && (
                <Box mb={5} data-testid="scriptbox">
                  Configure the required permission in your AWS account.
                  <Text mb={2}>
                    Open{' '}
                    <Link
                      href="https://console.aws.amazon.com/cloudshell/home"
                      target="_blank"
                    >
                      AWS CloudShell
                    </Link>{' '}
                    and copy and paste the command that configures the
                    permissions for you:
                  </Text>
                  <Box mb={2}>
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
              {scriptUrl && (
                <ButtonSecondary
                  mb={3}
                  onClick={() => setScriptUrl('')}
                  disabled={confirmed}
                >
                  Edit
                </ButtonSecondary>
              )}
              {showGenerateCommand && !scriptUrl && (
                <ButtonBorder
                  mb={3}
                  onClick={() => generateAwsOidcConfigIdpScript(validator)}
                  disabled={
                    (!requiresS3BucketWarning && (!s3Bucket || !s3Prefix)) ||
                    !roleArn
                  }
                >
                  Generate Command
                </ButtonBorder>
              )}
            </EditableBox>
          </DialogContent>
          <DialogFooter>
            {showGenerateCommand && scriptUrl && (
              <Box mb={1}>
                <CheckboxInput
                  role="checkbox"
                  type="checkbox"
                  name="checkbox"
                  data-testid="checkbox"
                  checked={confirmed}
                  onChange={e => {
                    setConfirmed(e.target.checked);
                  }}
                />
                I have ran the command
              </Box>
            )}

            {requiresS3BucketWarning && showS3BucketWarning ? (
              <S3BucketWarningBanner
                onClose={() => setShowS3BucketWarning(false)}
                onContinue={() => {
                  setShowS3BucketWarning(false);
                  handleEdit(validator);
                }}
                btnFlexWrap={true}
              />
            ) : (
              <>
                <ButtonPrimary
                  mr="3"
                  disabled={
                    isProcessing ||
                    (showGenerateCommand && !confirmed) ||
                    !changeDetected
                  }
                  onClick={() => {
                    if (requiresS3BucketWarning) {
                      setShowS3BucketWarning(true);
                    } else {
                      handleEdit(validator);
                    }
                  }}
                >
                  Save
                </ButtonPrimary>
                <ButtonSecondary disabled={isProcessing} onClick={close}>
                  Cancel
                </ButtonSecondary>
              </>
            )}
          </DialogFooter>
        </Dialog>
      )}
    </Validation>
  );
}

const EditableBox = styled(Box)`
  border-radius: ${p => p.theme.space[1]}px;
  border: 2px solid ${p => p.theme.colors.spotBackground[1]};
  background-color: ${p => p.theme.colors.spotBackground[0]};
`;
