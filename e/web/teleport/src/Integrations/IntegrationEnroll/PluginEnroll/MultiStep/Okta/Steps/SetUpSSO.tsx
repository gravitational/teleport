import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState } from 'react';
import { Link as RouterLink } from 'react-router-dom';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H1,
  Indicator,
  Link,
  Text,
} from 'design';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation, { type Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  createOktaPlugin,
  OktaSetupStepComplete,
  SSO_CONFIG,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { StyledBox } from 'e-teleport/Integrations/Shared';
import { pluginsService } from 'e-teleport/services/plugins';
import { createFetchPluginQueryKey } from 'e-teleport/services/plugins/hooks';
import useTeleportE from 'e-teleport/useTeleportE';
import { Redirect } from 'teleport/components/Router';
import { Resource } from 'teleport/services/resources';
import { withUnsupportedOktaPluginCreateErrorConversion } from 'teleport/services/version/unsupported';

const NEW_AUTH_CONNECTOR_VALUE = crypto.randomUUID();

export const SetUpSSO = () => {
  const ctx = useTeleportE();
  const { startFrom } = useOktaIntegrationSetUpContext();
  const [showPickConnector, setShowPickConnector] = useState(false);

  const queryClient = useQueryClient();

  const updatePlugin = useMutation({
    mutationFn: ({
      eventId,
      metadataUrl,
      connector,
    }: {
      eventId: string;
      metadataUrl: string | undefined;
      connector: Option | undefined;
    }) =>
      createOktaPlugin({
        eventId,
        enableAccessListSync: false,
        enableAppGroupsSync: false,
        enableUserSync: false,
        metadataUrl: metadataUrl,
        reuseConnector: connector?.value,
      })
        .catch(err => {
          const msg = getErrMessage(err);
          if (msg.includes('fetching SAML app entity metadata')) {
            throw new Error(
              'Failed to fetch SAML app entity metadata. Check the provided URL is valid and try again.'
            );
          }
          throw err;
        })
        .catch(withUnsupportedOktaPluginCreateErrorConversion)
        // refetch the plugin to put into the cache because the create plugin endpoint doesn't return plugin.status.details
        .then(() => pluginsService.fetchPlugin('okta')),
    onSuccess: data =>
      queryClient.setQueryData(createFetchPluginQueryKey('okta'), data),
  });

  const existingConnectors = useQuery({
    queryKey: ['existingConnectors'],
    queryFn: async ({ signal }) => {
      const authConnectors =
        await ctx.resourceService.fetchAuthConnectors(signal);

      const samlConnectors = (authConnectors?.connectors?.filter(
        conn => conn.kind === 'saml'
      ) ?? []) as Resource<'saml'>[];

      setShowPickConnector(samlConnectors?.length > 0);

      return samlConnectors;
    },
    gcTime: 0,
  });

  const onSubmit = useCallback(
    async ({
      metadataUrl,
      connector,
      validator,
    }: {
      validator?: Validator;
    } & (
      | { metadataUrl: string; connector?: undefined }
      | { connector: Option; metadataUrl?: undefined }
    )) => {
      if (connector?.value === NEW_AUTH_CONNECTOR_VALUE) {
        setShowPickConnector(false);
        return;
      }
      if (updatePlugin.isPending || (validator && !validator.validate())) {
        return;
      }

      const eventId = crypto.randomUUID();

      updatePlugin.mutate({
        eventId,
        metadataUrl,
        connector,
      });
    },
    [updatePlugin]
  );

  if (updatePlugin.isSuccess) {
    return <OktaSetupStepComplete config={SSO_CONFIG} />;
  }

  if (startFrom !== undefined) {
    return <Redirect to={cfg.oss.getIntegrationEnrollRoute('okta')} />;
  }

  if (existingConnectors.isPending) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  return (
    <Flex flexDirection="column" gap={5} maxWidth={900}>
      {existingConnectors.isError && (
        <Alert kind="outline-danger" mb={0}>
          {getErrMessage(existingConnectors.error)}
        </Alert>
      )}
      {showPickConnector ? (
        <PickExistingConnector
          connectors={(existingConnectors.data ?? []).map(conn => ({
            label: conn.name,
            value: conn.name,
          }))}
          error={updatePlugin.error}
          isError={updatePlugin.isError}
          onContinue={connector => onSubmit({ connector })}
        />
      ) : (
        <MetadataURLForm
          error={updatePlugin.error}
          isDisabled={updatePlugin.isPending}
          isError={updatePlugin.isError}
          onSubmit={onSubmit}
        />
      )}
    </Flex>
  );
};

const MetadataURLForm = ({
  error,
  isDisabled,
  isError,
  onSubmit,
}: {
  error: unknown;
  isDisabled: boolean;
  isError: boolean;
  onSubmit: ({
    metadataUrl,
    validator,
  }: {
    metadataUrl: string;
    validator: Validator;
  }) => void;
}) => {
  const [metadataUrl, setMetadataUrl] = useState('');

  return (
    <>
      <Box>
        <Header header="Configure SSO" />
      </Box>
      {isError && (
        <Alert kind="outline-danger" mb={0}>
          {getErrMessage(error)}
        </Alert>
      )}
      <StyledBox header="Step 1: Create an application in the Okta dashboard to allow Teleport to access Okta as an IdP provider">
        <Text>
          Follow the instructions in the Teleport documentation to{' '}
          <Link
            href="https://goteleport.com/docs/admin-guides/access-controls/sso/okta/#step-14-create--assign-groups"
            target="_blank"
          >
            create the required groups
          </Link>{' '}
          and{' '}
          <Link
            href="https://goteleport.com/docs/admin-guides/access-controls/sso/okta/#step-24-configure-okta"
            target="_blank"
          >
            create and configure
          </Link>{' '}
          the SAML application in the Okta dashboard.
        </Text>
      </StyledBox>
      <Validation>
        {({ validator }) => (
          <>
            <StyledBox header="Step 2: Copy and paste the 'Metadata URL' from Okta">
              <Text mb={4}>
                In Okta, go to your new Teleport SSO application and open the{' '}
                <b>Sign On</b> tab. From the <b>Sign on methods</b> section,
                copy the Metadata URL and paste it below:
              </Text>
              <FieldInput
                width="500px"
                label="Metadata URL"
                name={FormDataField.MetadataURL}
                value={metadataUrl}
                rule={requiredField('Please enter a valid Okta Metadata URL')}
                onChange={e => setMetadataUrl(e.target.value)}
                placeholder="https://your_okta_domain.okta.com/app/your_app_ID/sso/saml/metadata"
                disabled={isDisabled}
              />
            </StyledBox>
            <Flex flexDirection="row" alignItems="center" gap={3}>
              <ButtonPrimary
                onClick={() => onSubmit({ metadataUrl, validator })}
                disabled={isDisabled}
              >
                Continue
              </ButtonPrimary>
              <ButtonSecondary
                as={RouterLink}
                to={cfg.oss.getIntegrationEnrollRoute('okta')}
                disabled={isDisabled}
              >
                Back
              </ButtonSecondary>
            </Flex>
          </>
        )}
      </Validation>
    </>
  );
};

const PickExistingConnector = ({
  connectors,
  onContinue,
  isError,
  error,
}: {
  connectors: Option[];
  onContinue: (opt: Option) => void;
  isError: boolean;
  error: unknown;
}) => {
  const options = [...connectors];
  if (!options.some(o => o.value === 'okta')) {
    options.push({
      label: 'Create new connector "okta"...',
      value: NEW_AUTH_CONNECTOR_VALUE,
    });
  }
  const [selectedConnector, setSelectedConnector] = useState(options[0]);

  return (
    <>
      <Box>
        <H1 mb={2}>Choose Your Auth Connector</H1>
      </Box>
      {isError && (
        <Alert kind="danger" mb={0}>
          {getErrMessage(error)}
        </Alert>
      )}
      <Flex flexDirection="column" gap={3} maxWidth={300}>
        <Validation>
          <FieldSelect
            label="Select Connector"
            options={options}
            value={selectedConnector}
            onChange={opt => setSelectedConnector(opt)}
          />
        </Validation>
      </Flex>
      <Flex flexDirection="row" alignItems="center" gap={3}>
        <ButtonPrimary onClick={() => onContinue(selectedConnector)}>
          Next
        </ButtonPrimary>
        <ButtonSecondary
          as={RouterLink}
          to={cfg.oss.getIntegrationEnrollRoute('okta')}
        >
          Back
        </ButtonSecondary>
      </Flex>
    </>
  );
};
