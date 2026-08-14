import {
  useMutation,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from '@tanstack/react-query';
import { useCallback, useState } from 'react';
import { Link } from 'react-router';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  Image,
  Indicator,
  Text,
} from 'design';
import pamSuccess from 'design/assets/images/icons/success.png';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import Validation from 'shared/components/Validation';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { PluginIcon } from 'e-teleport/Integrations/IntegrationEnroll/IntegrationPick/PluginIcon';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { SCIMEnrolNoAuthConnectorsAlert } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/SCIM/Shared';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/SCIM/types';
import { StyledBox } from 'e-teleport/Integrations/Shared';
import { pluginsService } from 'e-teleport/services/plugins';
import { createFetchPluginQueryKey } from 'e-teleport/services/plugins/hooks';
import useTeleportE from 'e-teleport/useTeleportE';
import { addIndexToViews } from 'teleport/components/Wizard/flow';
import { Navigation } from 'teleport/components/Wizard/Navigation';
import type { Plugin } from 'teleport/services/integrations';
import { KindAuthConnectors, Resource } from 'teleport/services/resources';

enum ScimIntegrationStepType {
  AuthConnector = 'auth-connector',
  Credentials = 'credentials',
  Complete = 'complete',
}

const scimIntegrationSteps = addIndexToViews([
  {
    title: 'Choose Auth Connector',
  },
  {
    title: 'SCIM Credentials',
  },
  {
    title: 'Finished',
    hide: true,
  },
]);

// TODO(kiosion): Break 'kind' union into separate constants & types ('KindSAML', 'KindOIDC', etc.)
type SamlOrOidcConnector = Resource<Exclude<KindAuthConnectors, 'github'>>;

export const SCIMIntegrationSetUp = () => {
  const ctx = useTeleportE();
  const queryClient = useQueryClient();
  const [step, setStep] = useState<ScimIntegrationStepType>(
    ScimIntegrationStepType.AuthConnector
  );

  const authConnectors = useQuery({
    queryKey: ['authConnectors'],
    queryFn: async ({ signal }) => {
      const res = await ctx.resourceService.fetchAuthConnectors(signal);
      let hasSaml = false,
        hasOidc = false;
      const samlOrOidcConnectors: SamlOrOidcConnector[] = (
        res?.connectors || []
      ).reduce((acc, c) => {
        if (c.kind === 'saml') {
          hasSaml = true;
          acc.push(c);
        } else if (c.kind === 'oidc') {
          hasOidc = true;
          acc.push(c);
        }
        return acc;
      }, []);
      return samlOrOidcConnectors.map(conn => ({
        label:
          hasSaml && hasOidc
            ? `${conn.name} (${conn.kind.toUpperCase()})`
            : conn.name,
        value: conn,
      })) satisfies Option<SamlOrOidcConnector>[];
    },
    gcTime: 0,
  });

  const createIntegration = useMutation({
    mutationFn: (conn: SamlOrOidcConnector) => {
      const formData = new FormData();
      formData.set('type', 'scim');
      formData.set(FormDataField.ConnectorName, conn.name);
      formData.set(FormDataField.ConnectorKind, conn.kind);
      return pluginsService.createStaticAuthPlugin<'scim'>(formData);
    },
    onSuccess: data => {
      queryClient.setQueryData(createFetchPluginQueryKey('scim'), data);
      setStep(ScimIntegrationStepType.Credentials);
    },
  });

  const [selectedConnector, setSelectedConnector] = useState<
    Option<SamlOrOidcConnector> | undefined
  >();

  const onContinue = useCallback(() => {
    if (step === ScimIntegrationStepType.Credentials) {
      setStep(ScimIntegrationStepType.Complete);
      return;
    }

    if (!selectedConnector || createIntegration.isPending) {
      return;
    }

    return createIntegration.mutate(selectedConnector.value);
  }, [createIntegration, selectedConnector, step]);

  if (authConnectors.isPending) {
    return <CenteredLoading />;
  }

  if (step === ScimIntegrationStepType.Complete) {
    return <SetupComplete />;
  }

  return (
    <>
      <Box my={4}>
        <Navigation
          currentStep={step === ScimIntegrationStepType.AuthConnector ? 0 : 1}
          views={scimIntegrationSteps}
          startWithIcon={{
            title: 'SCIM Integration',
            component: <PluginIcon type="scim" size={16} />,
          }}
        />
      </Box>
      <Flex flexDirection="column" gap={4}>
        <Flex flexDirection="column" gap={5} maxWidth={900}>
          <Box>
            <Header header="Set Up SCIM" />
            <Text>
              The SCIM (System for Cross-domain Identity Management) integration
              allows different Identity Providers (IdPs) to push user and
              permission changes to Teleport in real-time.
            </Text>
          </Box>
          {createIntegration.isError && (
            <Alert kind="danger" mb={0}>
              {getErrMessage(createIntegration.error)}
            </Alert>
          )}
          {authConnectors.isError && (
            <Alert kind="danger" mb={0}>
              {getErrMessage(authConnectors.error)}
            </Alert>
          )}
          {authConnectors.isSuccess && !authConnectors.data?.length && (
            <SCIMEnrolNoAuthConnectorsAlert />
          )}
          {step === ScimIntegrationStepType.AuthConnector ? (
            <AuthConnectorStep
              query={authConnectors}
              selected={selectedConnector}
              onChange={setSelectedConnector}
            />
          ) : (
            <CredentialsStep data={createIntegration.data} />
          )}
        </Flex>
        <Flex flexDirection="row" alignItems="center" gap={3}>
          <ButtonPrimary
            onClick={onContinue}
            disabled={!selectedConnector || createIntegration.isPending}
          >
            Continue
          </ButtonPrimary>
          <ButtonSecondary
            as={Link}
            to={cfg.oss.getIntegrationEnrollRoute()}
            disabled={
              createIntegration.isPending ||
              step === ScimIntegrationStepType.Credentials
            }
          >
            Cancel
          </ButtonSecondary>
        </Flex>
      </Flex>
    </>
  );
};

const AuthConnectorStep = ({
  query,
  selected,
  onChange,
}: {
  query: UseQueryResult<Option<SamlOrOidcConnector>[]>;
  selected?: Option<SamlOrOidcConnector>;
  onChange: (value: Option<SamlOrOidcConnector>) => void;
}) => (
  <Flex flexDirection="column" gap={4} width="100%">
    <StyledBox header="Auth Connector">
      <Text>
        Map SCIM with an Auth Connector to ensure identity remains consistent
        between provisioning and authentication, and users from untrusted
        sources are not provisioned.
      </Text>
      {query.data?.length ? (
        <Validation>
          <FieldSelect
            label="Select Auth Connector"
            options={query.data}
            value={selected}
            onChange={onChange}
          />
        </Validation>
      ) : (
        <Text color="text.slightlyMuted">No Auth Connectors available.</Text>
      )}
    </StyledBox>
  </Flex>
);

const CredentialsStep = ({ data }: { data: Plugin<'scim'> }) => {
  return (
    <Flex flexDirection="column" gap={4} width="100%">
      <StyledBox
        header="Teleport SCIM Base URL"
        tooltip={
          'The endpoint provided by Teleport where your IdP should send SCIM requests'
        }
      >
        <Text>
          Copy and paste the URL below into the SCIM connector Base URL field in
          your Identity Provider:
        </Text>
        <TextSelectCopy
          bash={false}
          text={`${cfg.oss.baseUrl}/v1/webapi/scim/${data?.name}`}
        />
      </StyledBox>
      <StyledBox
        header="Teleport SCIM Token URL"
        tooltip={
          'The endpoint provided by Teleport where your IdP should obtain a JWT token for SCIM requests'
        }
      >
        <Text>
          Copy and paste the URL below into the SCIM connector Token URL field
          in your Identity Provider:
        </Text>
        <TextSelectCopy
          bash={false}
          text={`${cfg.oss.baseUrl}/v1/webapi/plugin/${data?.name}/token`}
        />
      </StyledBox>
      <StyledBox
        header="Teleport Client ID"
        tooltip={
          'The client ID used to obtain a JWT token for SCIM requests from your IdP'
        }
      >
        <Text>
          Copy and paste the value below into the client ID field in your
          Identity Provider:
        </Text>
        <TextSelectCopy
          bash={false}
          text={data?.credentials?.OAuthCredentials?.clientId}
          obfuscate
        />
      </StyledBox>
      <StyledBox
        header="Teleport Client Secret"
        tooltip={
          'The client secret used to obtain a JWT token for SCIM requests from your IdP'
        }
      >
        <Text>
          Copy and paste the value below into the client secret field in your
          Identity Provider:
        </Text>
        <TextSelectCopy
          bash={false}
          text={data?.credentials?.OAuthCredentials?.clientSecret}
          obfuscate
        />
      </StyledBox>
    </Flex>
  );
};

const SetupComplete = () => (
  <Flex flexDirection="column" alignItems="center" mt={6} gap={2}>
    <Image src={pamSuccess} maxWidth="120px" />
    <H2>SCIM Integration Completed Successfully</H2>
    <Box maxWidth="500px" textAlign="center" mb={1}>
      Go to your SCIM access list to view all the members enrolled.
    </Box>
    <Flex flexDirection="row" alignItems="center" gap={3}>
      <ButtonPrimary as={Link} to={cfg.getAccessListManagementRoute()}>
        Go to Access Lists
      </ButtonPrimary>
      <ButtonSecondary as={Link} to={cfg.oss.getIntegrationEnrollRoute()}>
        Back to Integrations
      </ButtonSecondary>
    </Flex>
  </Flex>
);

const CenteredLoading = () => (
  <Box textAlign="center" m={10}>
    <Indicator />
  </Box>
);
