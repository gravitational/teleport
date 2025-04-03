import { useCallback, useEffect, useState } from 'react';
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
import { Attempt, useAsync } from 'shared/hooks/useAsync';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  createOktaPlugin,
  OktaIntegrationLevel,
  OktaSetupStepComplete,
  StyledBox,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import useTeleportE from 'e-teleport/useTeleportE';
import { Redirect } from 'teleport/components/Router';
import { Resource } from 'teleport/services/resources';
import { withUnsupportedOktaPluginCreateErrorConversion } from 'teleport/services/version/unsupported';

const NEW_AUTH_CONNECTOR_VALUE = crypto.randomUUID();

export const SetUpSSO = () => {
  const ctx = useTeleportE();
  const { startFrom, setPlugin } = useOktaIntegrationSetUpContext();
  const [showPickConnector, setShowPickConnector] = useState(false);
  const [updatePluginAttempt, updatePlugin] = useAsync(
    useCallback(
      ({
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
          .catch(withUnsupportedOktaPluginCreateErrorConversion),
      []
    )
  );
  const [existingConnectorsAttempt, fetchExistingConnectors] = useAsync(
    useCallback(
      async () =>
        await ctx.resourceService.fetchAuthConnectors().then(res => {
          const samlConnectors = (res?.connectors?.filter(
            conn => conn.kind === 'saml'
          ) ?? []) as Resource<'saml'>[];
          setShowPickConnector(samlConnectors?.length > 0);
          return samlConnectors;
        }),
      [ctx.resourceService]
    )
  );

  useEffect(() => {
    if (existingConnectorsAttempt.status !== '') {
      return;
    }
    void fetchExistingConnectors();
  }, [existingConnectorsAttempt.status, fetchExistingConnectors]);

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
      if (
        updatePluginAttempt.status === 'processing' ||
        (validator && !validator.validate())
      ) {
        return;
      }

      const eventId = crypto.randomUUID();
      const [resp, err] = await updatePlugin({
        eventId,
        metadataUrl,
        connector,
      });
      if (err) {
        return;
      }

      setPlugin(resp);
      // TODO(kiosion): Capture enrol event after event is updated for step-by-step enrolment
    },
    [updatePluginAttempt.status, setPlugin, updatePlugin]
  );

  if (startFrom !== undefined) {
    return <Redirect to={cfg.oss.getIntegrationEnrollRoute('okta')} />;
  }

  if (updatePluginAttempt.status === 'success') {
    return <OktaSetupStepComplete step={OktaIntegrationLevel.SSO} />;
  }

  if (['', 'processing'].includes(existingConnectorsAttempt.status)) {
    return (
      <Box textAlign="center" m={10}>
        <Indicator />
      </Box>
    );
  }

  return (
    <Flex flexDirection="column" gap={5} maxWidth={900}>
      {existingConnectorsAttempt.status === 'error' && (
        <Alert kind="outline-danger" mb={0}>
          {getErrMessage(existingConnectorsAttempt.error)}
        </Alert>
      )}
      {showPickConnector ? (
        <PickExistingConnector
          connectors={existingConnectorsAttempt.data.map(conn => ({
            label: conn.name,
            value: conn.name,
          }))}
          onContinue={connector => onSubmit({ connector })}
          attempt={updatePluginAttempt}
        />
      ) : (
        <MetadataURLForm attempt={updatePluginAttempt} onSubmit={onSubmit} />
      )}
    </Flex>
  );
};

const MetadataURLForm = ({
  attempt,
  onSubmit,
}: {
  attempt: Attempt<unknown>;
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
      {attempt.status === 'error' && (
        <Alert kind="outline-danger" mb={0}>
          {getErrMessage(attempt.error)}
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
                disabled={attempt.status === 'processing'}
              />
            </StyledBox>
            <Flex flexDirection="row" alignItems="center" gap={3}>
              <ButtonPrimary
                onClick={() => onSubmit({ metadataUrl, validator })}
                disabled={attempt.status === 'processing'}
              >
                Submit SSO Configuration
              </ButtonPrimary>
              <ButtonSecondary
                as={RouterLink}
                to={cfg.oss.getIntegrationEnrollRoute('okta')}
                disabled={attempt.status === 'processing'}
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
  attempt,
}: {
  connectors: Option[];
  onContinue: (opt: Option) => void;
  attempt: Attempt<unknown>;
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
      {attempt.status === 'error' && (
        <Alert kind="danger" mb={0}>
          {getErrMessage(attempt.error)}
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
