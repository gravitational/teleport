import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  PropsWithChildren,
  useCallback,
  useState,
  type ComponentProps,
  type ComponentType,
} from 'react';
import { Link as RouterLink } from 'react-router-dom';

import {
  Alert,
  Box,
  Button,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Indicator,
  Link,
  Mark,
  Text,
  Toggle,
} from 'design';
import { CollapsibleInfoSection } from 'design/CollapsibleInfoSection';
import { Question } from 'design/Icon';
import type { IconProps } from 'design/Icon/Icon';
import { HoverTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import Validation, { type Validator } from 'shared/components/Validation';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  BulletList,
  ListItem,
  NumberedList,
  OktaIntegrationStepType,
  OktaSetupStepComplete,
  USER_SYNC_CONFIG,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import type { OktaIntegrationStepFormProps } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Steps/props';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { StyledBox } from 'e-teleport/Integrations/Shared';
import { pluginsService } from 'e-teleport/services/plugins';
import { createFetchPluginQueryKey } from 'e-teleport/services/plugins/hooks';
import { Redirect } from 'teleport/components/Router';
import { withUnsupportedOktaPluginUpdateErrorConversion } from 'teleport/services/version/unsupported';

export const SetUpUserSync = () => {
  const { completedStepTypes, getPreviousStep, plugin, startFrom } =
    useOktaIntegrationSetUpContext();

  if (
    startFrom === OktaIntegrationStepType.UserSync ||
    startFrom === OktaIntegrationStepType.AppGroupSync
  ) {
    return <Redirect to={cfg.oss.getIntegrationEnrollRoute('okta')} />;
  }

  const previousStep = getPreviousStep(OktaIntegrationStepType.UserSync);
  const previousStepType = completedStepTypes.includes(previousStep?.type)
    ? undefined
    : previousStep?.type;

  return <UserSyncForm plugin={plugin} previousStepType={previousStepType} />;
};

export const UserSyncForm = ({
  isEditing,
  plugin,
  previousStepType,
}: OktaIntegrationStepFormProps) => {
  const [clientId, setClientId] = useState<string>('');
  const [bidirectionalSync, setBidirectionalSync] = useState<boolean>(
    !isEditing ? true : !!plugin?.spec?.enableBidirectionalSync
  );
  const [showingSetupSteps, setShowingSetupSteps] = useState(
    !(isEditing && plugin?.spec?.credentialsInfo?.hasConfiguredOauthCredentials)
  );

  const queryClient = useQueryClient();

  const updatePlugin = useMutation({
    mutationFn: () =>
      pluginsService
        .updatePlugin({
          plugin: 'okta',
          okta: {
            enableUserSync: true,
            enableBidirectionalSync: bidirectionalSync,
            assignDefaultRoles: !!plugin?.spec?.assignDefaultRoles,
            clientID: clientId?.trim()?.length ? clientId.trim() : undefined,
            enableAppGroupSync: !!plugin.spec?.enableAppGroupSync,
            enableAccessListSync: !!plugin.spec?.enableAccessListSync,
            enableSystemLogExport: !!plugin.spec?.enableSystemLogExport,
            defaultOwners: plugin.spec?.defaultOwners,
            appFilters:
              plugin.status?.details?.accessListsSyncDetails?.appFilters,
            groupFilters:
              plugin.status?.details?.accessListsSyncDetails?.groupFilters,
          },
        })
        .catch(withUnsupportedOktaPluginUpdateErrorConversion)
        .catch(err => {
          const msg = getErrMessage(err);
          if (
            msg
              .toLowerCase()
              .includes("invalid value for 'client_id' parameter")
          ) {
            throw new Error('Please enter a valid Client ID');
          }
          throw err;
        }),
    onSuccess: data =>
      queryClient.setQueryData(createFetchPluginQueryKey('okta'), data),
  });

  const onSubmit = useCallback(
    (validator: Validator) => {
      if (updatePlugin.isPending || !validator.validate()) {
        return;
      }

      updatePlugin.mutate();
    },
    [updatePlugin]
  );

  if (updatePlugin.isSuccess) {
    if (isEditing) {
      return (
        <Redirect to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')} />
      );
    }

    return <OktaSetupStepComplete config={USER_SYNC_CONFIG} />;
  }

  const isConfigured =
    isEditing && plugin?.spec?.credentialsInfo?.hasConfiguredOauthCredentials;

  // If editing, and the plugin has previously been configured with a ClientID,
  // collapse the first two steps by default as the user has already completed them.
  const MaybeCollapsibleInfoSection = isConfigured
    ? CollapsibleInfoSection
    : ({ children }: PropsWithChildren) => <>{children}</>;

  return (
    <Flex flexDirection="column" gap={5} maxWidth={900}>
      <Box>
        <Header header={isConfigured ? 'Edit User Sync' : 'Sync Users'} />
        <Text>
          User sync will ensure that Okta users persist in Teleport after they
          log out, so you always have a full view of your team’s access.
        </Text>
      </Box>
      <MaybeCollapsibleInfoSection
        openLabel="Show Okta setup steps"
        closeLabel="Hide Okta setup steps"
        onClick={v => setShowingSetupSteps(v)}
        maxWidth="800px"
      >
        <StyledBox
          header="Step 1: Create 'API Services' OAuth App in Okta"
          mb={isConfigured ? 3 : 0}
        >
          <NumberedList>
            <ListItem>
              <Text>
                In the Okta dashboard go to the navigation bar and click{' '}
                <b>Applications -&gt; Applications</b>, then{' '}
                <b>Create App Integration</b>.
              </Text>
            </ListItem>
            <ListItem mb={0}>
              <Text>
                Select <b>API Services</b> as the type of application and click{' '}
                <b>Next</b>. Give it a name and click <b>Save</b>.
              </Text>
            </ListItem>
          </NumberedList>
        </StyledBox>
        <StyledBox header="Step 2: Configure App Settings">
          <NumberedList>
            <ListItem>
              <Text>
                Go to the <b>General</b> tab of your new app. Click <b>Edit</b>{' '}
                in the <b>Client Credentials</b> section.
              </Text>
            </ListItem>
            <ListItem>
              <Text>
                Select <b>Public key / Private key</b> as the client
                authentication method.
              </Text>
            </ListItem>
            <ListItem>
              <OktaJwksStep />
            </ListItem>
            <ListItem>
              <Text>
                Click <b>Save</b> to update the Client Credential settings.
              </Text>
            </ListItem>
            <ListItem mb={0}>
              <Text>
                Under <b>General Settings</b>, click <b>Edit</b>, uncheck{' '}
                <b>Require Demonstrating Proof of Possession</b>, then click{' '}
                <b>Save</b>.
              </Text>
            </ListItem>
          </NumberedList>
        </StyledBox>
      </MaybeCollapsibleInfoSection>
      <StyledBox
        header={
          isEditing && !showingSetupSteps
            ? 'Update Granted Okta API Scopes'
            : 'Step 3: Grant Okta API Scopes'
        }
      >
        <NumberedList>
          <ListItem>
            <Text>
              Decide whether to enable Access Requests in Teleport. If enabled,
              Teleport will take ownership of imported Okta groups and modify
              their memberships based on Access Requests and changes made to
              Access Lists. Disable this if you want Teleport to have read-only
              access to your Okta organization.
            </Text>
            <Toggle
              isToggled={bidirectionalSync}
              onToggle={() => setBidirectionalSync(v => !v)}
              size="large"
            >
              <Text mb="1px" ml={2} fontSize={2}>
                Enable Access Requests in Teleport
              </Text>
            </Toggle>
          </ListItem>
          <GrantScopesSteps
            readOnly={!bidirectionalSync}
            showResourceSetSteps={!isEditing || showingSetupSteps}
          />
        </NumberedList>
      </StyledBox>
      <Validation>
        {({ validator }) => (
          <>
            <StyledBox
              header={
                isEditing && !showingSetupSteps
                  ? 'Update Client ID'
                  : 'Step 4: Provide Client ID'
              }
            >
              {(!isEditing || showingSetupSteps) && (
                <Text>
                  Go to the <b>General</b> tab of your app. Under{' '}
                  <b>Client Credentials</b>, copy the <b>Client ID</b> value and
                  paste it below.
                </Text>
              )}
              {isConfigured && !showingSetupSteps && (
                <Text>
                  A Client ID is already configured – Only enter a new value
                  here if you want to change it.
                </Text>
              )}
              <FieldInput
                width="500px"
                label="Client ID"
                name={FormDataField.MetadataURL}
                value={clientId}
                rule={(value?: string) => () =>
                  value?.trim?.()?.length > 1 ||
                  (!value &&
                    plugin?.spec?.credentialsInfo
                      ?.hasConfiguredOauthCredentials)
                    ? { valid: true }
                    : {
                        valid: false,
                        message: 'Please enter a valid Client ID',
                      }
                }
                onChange={e => setClientId(e.target.value)}
                placeholder={
                  plugin?.spec?.credentialsInfo?.hasConfiguredOauthCredentials
                    ? '••••••••••••'
                    : '0oa3s1...fx25d6'
                }
                disabled={updatePlugin.isPending}
                mb={1}
              />
            </StyledBox>
            {updatePlugin.isError && (
              <Box maxWidth={800}>
                <Alert kind="danger" mb={0}>
                  {getErrMessage(updatePlugin.error)}
                </Alert>
              </Box>
            )}
            {/* If not in initial setup, and enabling bidirectional sync, remind about additional required Okta API scopes. */}
            {isEditing &&
              bidirectionalSync &&
              !plugin.spec?.enableBidirectionalSync && (
                <Box maxWidth={800}>
                  <Alert kind="outline-info" mb={0}>
                    <Text>
                      Enabling support for Access Requests requires additional
                      scopes for Okta API access. Please ensure you have granted
                      the necessary scopes.
                    </Text>
                  </Alert>
                </Box>
              )}
            <Flex flexDirection="row" alignItems="center" gap={3}>
              <ButtonPrimary
                onClick={() => onSubmit(validator)}
                disabled={updatePlugin.isPending}
              >
                {isEditing ? 'Save Changes' : 'Continue'}
              </ButtonPrimary>
              <ButtonSecondary
                as={RouterLink}
                to={
                  isEditing
                    ? cfg.oss.getIntegrationStatusRoute('okta', 'okta')
                    : cfg.oss.getIntegrationEnrollRoute(
                        'okta',
                        previousStepType
                      )
                }
                disabled={updatePlugin.isPending}
              >
                {isEditing ? 'Cancel' : 'Back'}
              </ButtonSecondary>
            </Flex>
          </>
        )}
      </Validation>
    </Flex>
  );
};

type RequiredScope = [string, string];

const requiredScopes: RequiredScope[] = [
  [
    'okta.apps.manage',
    'Required to create and manage Apps in your Okta organization',
  ],
  [
    'okta.apps.read',
    'Required to read information about Apps in your Okta organization',
  ],
  [
    'okta.groups.manage',
    'Required to manage existing groups in your Okta organization',
  ],
  [
    'okta.groups.read',
    'Required to read information about groups and their members in your Okta organization',
  ],
  [
    'okta.users.manage',
    "Required to create new users and to manage all users' profile and credentials information",
  ],
  [
    'okta.users.read',
    "Required to read the existing users' profiles and credentials",
  ],
] as const;

const requiredRolePermissions = [
  [
    'User permissions',
    [
      'View users and their details',
      "Edit users' group membership",
      "Edit users' application assignments",
    ],
  ],
  ['Group permissions', ['Manage groups']],
  [
    'Application permissions',
    [
      'View applications and their details',
      "Edit applications' user assignments",
    ],
  ],
] as const;

const HelpButton = ({
  label,
  icon: Icon = Question,
  ...props
}: {
  label: string;
  icon?: ComponentType<IconProps>;
} & ComponentProps<typeof Button>) => (
  <Button
    {...props}
    type="button"
    intent="neutral"
    size="small"
    alignSelf="flex-start"
  >
    <Icon size="small" />
    <Text ml={2}>{label}</Text>
  </Button>
);

export const RequiredPermissionItem = ({
  text,
  description,
}: {
  text: string;
  description: string;
}) => (
  <ListItem my={1} width="fit-content">
    <HoverTooltip tipContent={description} position="right">
      <Text>
        <Mark>{text}</Mark>
      </Text>
    </HoverTooltip>
  </ListItem>
);

const GrantScopesSteps = ({
  readOnly,
  showResourceSetSteps,
}: {
  readOnly: boolean;
  showResourceSetSteps: boolean;
}) => {
  const scopes = requiredScopes.filter(
    ([scope]) => !readOnly || scope.endsWith('read')
  );

  return (
    <>
      <ListItem>
        <Text>
          Go to the <b>Okta API Scopes</b> tab of your app and grant the
          following scopes:
        </Text>
        <CollapsibleInfoSection
          size="small"
          openLabel="Show more"
          closeLabel="Show less"
          defaultOpen
        >
          <BulletList>
            {scopes.map(([scope, desc]) => (
              <RequiredPermissionItem
                key={scope}
                text={scope}
                description={desc}
              />
            ))}
          </BulletList>
        </CollapsibleInfoSection>
      </ListItem>
      {showResourceSetSteps && (
        <>
          <ListItem>
            <Text>
              Follow Okta&#39;s documentation to{' '}
              <Link
                href="https://help.okta.com/en-us/content/topics/security/custom-admin-role/create-resource-set.htm"
                target="_blank"
              >
                create a resource set
              </Link>{' '}
              constrained to all Users, Groups, and Applications, then{' '}
              <Link
                href="https://support.okta.com/help/s/article/How-to-Create-Custom-Admin-Roles"
                target="_blank"
              >
                create a custom admin role
              </Link>{' '}
              with the following permissions:
            </Text>
            <CollapsibleInfoSection
              size="small"
              openLabel="Show more"
              closeLabel="Show less"
            >
              {requiredRolePermissions.map(([title, roles]) => (
                <Box key={title}>
                  <Text mb={1} bold>
                    {title}
                  </Text>
                  <BulletList>
                    {roles.map(role => (
                      <ListItem key={role}>
                        <Text>{role}</Text>
                      </ListItem>
                    ))}
                  </BulletList>
                </Box>
              ))}
            </CollapsibleInfoSection>
          </ListItem>
          <ListItem mb={0}>
            <Text>
              Go to the <b>Admin Roles</b> tab of your app. Click{' '}
              <b>Edit Assignments</b>, and select the custom admin role you
              created, along with the resource set you created. Click{' '}
              <b>Save</b>.
            </Text>
          </ListItem>
        </>
      )}
    </>
  );
};

const OktaJwksStep = () => {
  const [showManualKeyInfo, setShowManualKeyInfo] = useState(false);

  const jwks = useQuery({
    queryKey: ['oktaJwks'],
    queryFn: () =>
      fetch(`${cfg.oss.baseUrl}/v1/.well-known/jwks-okta`)
        .then(res => res.json())
        .then(data => JSON.stringify(data.keys[0])),
    enabled: showManualKeyInfo,
  });

  const refetch = useCallback(() => {
    void jwks.refetch();
  }, [jwks]);

  const shared = (
    <HelpButton
      onClick={() => setShowManualKeyInfo(v => !v)}
      label={`If your cluster is ${!showManualKeyInfo ? 'private' : 'public'}`}
      mb={2}
    />
  );

  if (showManualKeyInfo) {
    return (
      <>
        <Text mb={2}>
          Under <b>Public Keys</b>, choose <b>Save keys in Okta</b>. Click{' '}
          <b>Add Key</b>, then copy and paste the following key:
        </Text>
        {jwks.isPending && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
        {jwks.isSuccess && (
          <TextSelectCopy text={jwks.data} bash={false} mb={2} />
        )}
        {jwks.isError && (
          <>
            <Alert
              kind="danger"
              mb={0}
              primaryAction={{
                content: 'Retry',
                onClick: refetch,
              }}
            >
              Failed to fetch key: {getErrMessage(jwks.error)}
            </Alert>
            <Text>
              If fetching the key fails, you may manually retrieve it by
              visiting{' '}
              <Link
                href={`${cfg.oss.baseUrl}/v1/.well-known/jwks-okta`}
                target="_blank"
              >{`${cfg.oss.baseUrl}/v1/.well-known/jwks-okta`}</Link>
              , then copying the first item from the <mark>keys</mark> array.
            </Text>
          </>
        )}
        {shared}
      </>
    );
  }

  return (
    <>
      <Text mb={2}>
        Under <b>Public Keys</b>, choose{' '}
        <b>Use a URL to fetch keys dynamically</b>, then copy and paste the
        following URL:
      </Text>
      <TextSelectCopy
        text={`${cfg.oss.baseUrl}/v1/.well-known/jwks-okta`}
        bash={false}
        mb={2}
      />
      {shared}
    </>
  );
};
