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
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonBorder,
  ButtonPrimary,
  ButtonSecondary,
  Link,
  Text,
} from 'design';
import { OutlineInfo, OutlineWarn } from 'design/Alert/Alert';
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

import { S3BucketConfiguration } from './Enroll/AwsOidc/S3BucketConfiguration';
import { EditableIntegrationFields } from './Operations/useIntegrationOperation';

type Props = {
  close(): void;
  edit(
    integration: IntegrationAwsOidc,
    req: EditableIntegrationFields
  ): Promise<void>;
  integration: IntegrationAwsOidc;
};

export function EditAwsOidcIntegrationDialog(props: Props) {
  const { close, edit, integration } = props;
  const [updateAttempt, runUpdate] = useAsync(async () => {
    await edit(integration, { kind: IntegrationKind.AwsOidc, roleArn });
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

  const s3Bucket = integration.spec.issuerS3Bucket;
  const s3Prefix = integration.spec.issuerS3Prefix;
  const showReadonlyS3Fields = s3Bucket || s3Prefix;

  const isProcessing = updateAttempt.status === 'processing';
  const showGenerateCommand =
    integration.spec.roleArn !== roleArn || showReadonlyS3Fields;

  const changeDetected = integration.spec.roleArn !== roleArn;

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
            {updateAttempt.status === 'error' && (
              <Alert children={updateAttempt.statusText} />
            )}
            <FieldInput
              label="Integration Name"
              value={integration.name}
              readonly={true}
            />
            <EditableBox px={3} pt={2} mt={2}>
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
                disabled={!!scriptUrl}
              />
              {showReadonlyS3Fields && !scriptUrl && (
                <>
                  <S3BucketConfiguration
                    s3Bucket={s3Bucket}
                    s3Prefix={s3Prefix}
                  />
                  <OutlineWarn>
                    Using an S3 bucket to store the OpenID Configuration is not
                    recommended. Reconfiguring this integration is suggested
                    (this will not break existing features).
                  </OutlineWarn>
                </>
              )}
              {scriptUrl && (
                <Box mb={5} data-testid="scriptbox">
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
                    {showReadonlyS3Fields && (
                      <OutlineInfo mt={3} linkColor="buttons.link.default">
                        <Box>
                          After running the command, delete the previous{' '}
                          <Link
                            target="_blank"
                            href={`https://console.aws.amazon.com/iam/home#/identity_providers/details/OPENID/${getIdpArn({ s3Bucket, s3Prefix, roleArn: integration.spec.roleArn })}`}
                          >
                            identity provider
                          </Link>{' '}
                          along with its{' '}
                          <Link
                            target="_blank"
                            href={`https://console.aws.amazon.com/s3/buckets/${s3Bucket}`}
                          >
                            S3 bucket
                          </Link>{' '}
                          from your AWS console.
                        </Box>
                      </OutlineInfo>
                    )}
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
              {!scriptUrl && showGenerateCommand && (
                <ButtonBorder
                  mb={3}
                  onClick={() =>
                    generateAwsOidcConfigIdpScript(
                      validator,
                      AwsOidcPolicyPreset.Unspecified
                    )
                  }
                  disabled={!roleArn}
                >
                  Reconfigure
                </ButtonBorder>
              )}
            </EditableBox>
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
            <ButtonPrimary
              mr="3"
              disabled={
                isProcessing ||
                (showGenerateCommand && !confirmed) ||
                (!showReadonlyS3Fields && !changeDetected)
              }
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

const EditableBox = styled(Box)`
  border-radius: ${p => p.theme.space[1]}px;
  border: 2px solid ${p => p.theme.colors.spotBackground[1]};
  background-color: ${p => p.theme.colors.spotBackground[0]};
`;

function getIdpArn({
  s3Bucket,
  s3Prefix,
  roleArn,
}: {
  s3Bucket: string;
  s3Prefix: string;
  roleArn: string;
}) {
  const { awsAccountId } = splitAwsIamArn(roleArn);
  const arn = `arn:aws:iam::${awsAccountId}:oidc-provider/${s3Bucket}.s3.amazonaws.com/${s3Prefix}`;
  return encodeURIComponent(arn);
}
