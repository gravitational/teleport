import React, { useEffect, useState } from 'react';
import styled from 'styled-components';

import {
  Alert,
  ButtonPrimary,
  Text,
  Link,
  Box,
  Flex,
  LabelInput,
  ButtonBorder,
} from 'design';
import Validation, { Validator } from 'shared/components/Validation';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import useAttempt from 'shared/hooks/useAttemptNext';

import * as Icons from 'design/Icon';

import { pluginsService } from 'e-teleport/services/plugins';

import cfg from 'e-teleport/config';

import { usePlugin } from '../../MultiStep/usePlugin';

import { Header } from '../Shared';

import { FormDataField } from './types';
export function RunScript() {
  const { formData, nextStep, setInstalledPlugin } = usePlugin();
  const accessGraphEnabled = !!formData
    .get(FormDataField.AccessGraph)
    ?.toString();
  const scriptUrl = cfg.getAzureOidcConfigureScriptUrl({
    authConnectorName: formData.get(FormDataField.AuthConnectorName).toString(),
    accessGraph: accessGraphEnabled,
  });

  const [tenantId, setTenantId] = useState('');
  const [clientId, setClientId] = useState('');

  const fileInput = React.createRef<HTMLInputElement>();
  const [fileName, setFileName] = useState('');
  const [fileHasError, setFileHasError] = useState(false);

  const { attempt, run } = useAttempt();

  function handleSubmit(validator: Validator) {
    let valid = true;
    if (accessGraphEnabled) {
      setFileHasError(!fileName);
      valid = !!fileName;
    }

    if (!validator.validate()) {
      valid = false;
    }

    if (!valid) return;

    formData.set(FormDataField.TenantId, tenantId);
    formData.set(FormDataField.ClientId, clientId);

    run(async () => {
      const resp = await pluginsService.createPlugin(formData);
      setInstalledPlugin(resp);
      nextStep();
    });
  }

  useEffect(() => {
    if (fileName) setFileHasError(false);
  }, [fileName]);

  function handleFileChange(e) {
    e.stopPropagation();
    e.preventDefault();

    const file = e.target.files[0];
    setFileName(file.name);
    formData.set(FormDataField.AccessGraphCache, file);
  }

  return (
    <Box width="800px">
      <Box mb={3}>
        <Header header="Set up permissions" />
        <Text>
          Teleport will configure an Azure enterprise application in order to
          access your directory data.
        </Text>
      </Box>
      <Flex flexDirection="column" gap={4} width="100%">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <StyledBox>
          <Text my={1} bold>
            Step 1: Run the setup script
          </Text>
          <Text mb={2}>
            Using Google Chrome or Safari, open{' '}
            <Link href="https://shell.azure.com" target="_blank">
              Azure Cloud Shell
            </Link>{' '}
            (Bash) and copy and paste the command that configures the
            permissions for you:
          </Text>
          <Flex flexDirection="column" gap={1}>
            <TextSelectCopyMulti
              lines={[
                {
                  text: `bash -c "$(curl '${scriptUrl}')"`,
                },
              ]}
            />
            <Text typography="subtitle" bold mt={2}>
              Considerations:
            </Text>
            <Flex as="ul" flexDirection="column" gap={1} pl={3} my={0}>
              <li>
                Azure privileged administrator permissions are required to set
                up the integration.
              </li>
              <li>Make sure to use the Bash environment in the Cloud Shell.</li>
              <li>
                The script will invoke <code>az login</code> to acquire
                necessary authentication for setting up the Azure enterprise
                application. Teleport never persists your credentials.
              </li>
              <li>
                Azure Cloud Shell has been reported to have connection issues
                with Mozilla Firefox.
              </li>
            </Flex>
          </Flex>
        </StyledBox>
        <Validation>
          {({ validator }) => (
            <>
              <StyledBox>
                <Text my={1} bold>
                  Step 2: Provide the script output values
                </Text>
                <Text mb={2}>
                  After the script is finished, it will output the remaining
                  information necessary to finish onboarding the integration.{' '}
                </Text>
                <FieldInput
                  disabled={attempt.status === 'processing'}
                  rule={requiredField('Tenant ID')}
                  value={tenantId}
                  label="Tenant ID"
                  placeholder="Tenant ID"
                  onChange={e => setTenantId(e.target.value)}
                />
                <FieldInput
                  disabled={attempt.status === 'processing'}
                  rule={requiredField('Client ID')}
                  value={clientId}
                  label="Client ID"
                  placeholder="Client ID"
                  onChange={e => setClientId(e.target.value)}
                />
                {accessGraphEnabled && (
                  <Flex alignItems="center" gap={3} mb={2} mt={5}>
                    <ButtonBorder
                      gap={2}
                      onClick={() => fileInput.current.click()}
                      size="large"
                      textTransform="none"
                      px={3}
                      disabled={attempt.status === 'processing'}
                    >
                      Click to upload the cache.json file{' '}
                      <Icons.Upload size="small" />
                    </ButtonBorder>

                    <FileLabel fontSize={2} hasError={fileHasError}>
                      {fileHasError ? 'Cache file must be specified' : fileName}
                    </FileLabel>
                    <input
                      disabled={attempt.status === 'processing'}
                      accept=".json"
                      style={{ display: 'none' }}
                      type="file"
                      onChange={handleFileChange}
                      ref={fileInput}
                      data-testid="access-graph-cache"
                    />
                  </Flex>
                )}
              </StyledBox>
              <Flex>
                <ButtonPrimary
                  disabled={attempt.status === 'processing'}
                  mt={1}
                  onClick={() => handleSubmit(validator)}
                >
                  Finish
                </ButtonPrimary>
              </Flex>
            </>
          )}
        </Validation>
      </Flex>
    </Box>
  );
}

const FileLabel = styled(LabelInput)`
  width: auto;
  font-size: ${props => props.theme.fontSizes[2]}px;
  margin-bottom: 0;
`;

const StyledBox = styled(Box).attrs({
  p: 4,
  borderRadius: 3,
  maxWidth: '800px',
})`
  background-color: ${props => props.theme.colors.spotBackground[0]};
`;
