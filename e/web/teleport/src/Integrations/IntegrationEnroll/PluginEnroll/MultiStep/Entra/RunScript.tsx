import { useState } from 'react';
import styled from 'styled-components';

import { Alert, Box, ButtonPrimary, Flex, Link, Text } from 'design';
import { ButtonFileUpload } from 'shared/components/ButtonFileUpload';
import FieldInput from 'shared/components/FieldInput';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';

import cfg from 'e-teleport/config';
import { pluginsService } from 'e-teleport/services/plugins';

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
  const [selectedFile, setSelectedFile] = useState<File>(null);

  const [showFileValidationError, setShowFileValidationError] =
    useState<boolean>(false);

  const { attempt, run } = useAttempt();

  function handleSubmit(validator: Validator) {
    let valid = true;
    if (accessGraphEnabled) {
      setShowFileValidationError(!selectedFile?.name);
      valid = !!selectedFile?.name;
    }

    if (!validator.validate()) {
      valid = false;
    }

    if (!valid) return;

    formData.set(FormDataField.TenantId, tenantId);
    formData.set(FormDataField.ClientId, clientId);
    if (accessGraphEnabled) {
      formData.set(FormDataField.AccessGraphCache, selectedFile);
    }

    run(async () => {
      const resp = await pluginsService.createPlugin(formData);
      setInstalledPlugin(resp);
      nextStep();
    });
  }

  function onFileSelect(f: File) {
    setShowFileValidationError(false);
    setSelectedFile(f);
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
            <Text bold mt={2}>
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
                  <Box mb={2} mt={5}>
                    <ButtonFileUpload
                      text="Click to upload the cache.json file"
                      errorMessage="Cache file must be specified"
                      accept=".json"
                      onFileSelect={onFileSelect}
                      showValidationError={showFileValidationError}
                      disabled={attempt.status === 'processing'}
                      buttonSize="large"
                      buttonIconSize="small"
                    />
                  </Box>
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

const StyledBox = styled(Box).attrs({
  p: 4,
  borderRadius: 3,
  maxWidth: '800px',
})`
  background-color: ${props => props.theme.colors.spotBackground[0]};
`;
