import {
  ComponentProps,
  ComponentType,
  PropsWithChildren,
  useCallback,
  useEffect,
  useState,
} from 'react';
import { Link as RouterLink } from 'react-router-dom';

import {
  Alert,
  Box,
  Button,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  H2,
  Indicator,
  Link,
  Mark,
  Text,
} from 'design';
import { ChevronDown, ChevronUp, Question } from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { HoverTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import { TextSelectCopy } from 'shared/components/TextSelectCopy';
import Validation, { type Validator } from 'shared/components/Validation';
import { useAsync } from 'shared/hooks/useAsync';
import { getErrMessage } from 'shared/utils/errorType';

import cfg from 'e-teleport/config';
import { useOktaIntegrationSetUpContext } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUpContext';
import {
  BulletList,
  ListItem,
  NumberedList,
  OktaIntegrationLevel,
  OktaSetupStepComplete,
  StyledBox,
} from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/Shared';
import { FormDataField } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/types';
import { Header } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Shared';
import { pluginsService } from 'e-teleport/services/plugins';
import { Redirect } from 'teleport/components/Router';
import type { Plugin, PluginOktaSpec } from 'teleport/services/integrations';
import { PluginStatusOkta } from 'teleport/services/integrations/oktaStatusTypes';
import { withUnsupportedOktaPluginUpdateErrorConversion } from 'teleport/services/version/unsupported';

export const SetUpUserSync = () => {
  const { plugin, setPlugin, startFrom } = useOktaIntegrationSetUpContext();

  if (
    startFrom === OktaIntegrationLevel.USER_SYNC ||
    startFrom === OktaIntegrationLevel.APP_GROUP_SYNC
  ) {
    return <Redirect to={cfg.oss.getIntegrationEnrollRoute('okta')} />;
  }

  return <UserSyncForm plugin={plugin} setPlugin={setPlugin} />;
};

export const UserSyncForm = ({
  plugin,
  setPlugin,
  isEditing = false,
}: {
  plugin: Plugin<PluginOktaSpec, PluginStatusOkta>;
  setPlugin: (plugin: Plugin<PluginOktaSpec, PluginStatusOkta>) => void;
  isEditing?: boolean;
}) => {
  const [clientId, setClientId] = useState<string | undefined>('');
  const [updatePluginAttempt, updatePlugin] = useAsync(
    useCallback(
      () =>
        pluginsService
          .updatePlugin({
            plugin: 'okta',
            okta: {
              enableUserSync: true,
              clientID: clientId.trim(),
              enableAppGroupSync: !!plugin.spec?.enableAppGroupSync,
              enableAccessListSync: !!plugin.spec?.enableAccessListSync,
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
      [plugin, clientId]
    )
  );

  const onSubmit = async (validator: Validator) => {
    if (updatePluginAttempt.status === 'processing' || !validator.validate()) {
      return;
    }

    const [resp, err] = await updatePlugin();
    if (!err) {
      setPlugin(resp);
    }
  };

  if (updatePluginAttempt.status === 'success') {
    return isEditing ? (
      <Redirect to={cfg.oss.getIntegrationStatusRoute('okta', 'okta')} />
    ) : (
      <OktaSetupStepComplete step={OktaIntegrationLevel.USER_SYNC} />
    );
  }

  return (
    <Flex flexDirection="column" gap={5} maxWidth={900}>
      <Box>
        <Header header="Sync Users" />
        <Text>
          User sync will ensure that Okta users persist in Teleport after they
          log out, so you always have a full view of your team’s access.
        </Text>
      </Box>
      <StyledBox>
        <H2 mb={1}>Step 1: Create &#34;API Services&#34; OAuth App in Okta</H2>
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
      <StyledBox>
        <H2 mb={1}>Step 2: Configure App Settings</H2>
        <NumberedList>
          <ListItem>
            <Text>
              Go to the <b>General</b> tab of your new app. Click <b>Edit</b> in
              the <b>Client Credentials</b> section.
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
      <StyledBox>
        <GrantScopesStep />
      </StyledBox>
      <Validation>
        {({ validator }) => (
          <>
            <StyledBox>
              <H2 mb={1}>Step 4: Provide Client ID</H2>
              <Text>
                Go back to the <b>General</b> tab of your app. Under{' '}
                <b>Client Credentials</b>, copy the <b>Client ID</b> value and
                paste it below.
              </Text>
              <FieldInput
                width="500px"
                label="Client ID"
                name={FormDataField.MetadataURL}
                value={clientId}
                rule={(value?: string) => () =>
                  value?.trim?.()?.length > 1
                    ? { valid: true }
                    : {
                        valid: false,
                        message: 'Please enter a valid Client ID',
                      }
                }
                onChange={e => setClientId(e.target.value)}
                placeholder="0oa3s1...fx25d6"
                disabled={updatePluginAttempt.status === 'processing'}
                mb={1}
              />
            </StyledBox>
            {updatePluginAttempt.status === 'error' && (
              <Alert kind="danger" mb={0}>
                {getErrMessage(updatePluginAttempt.error)}
              </Alert>
            )}
            <Flex flexDirection="row" alignItems="center" gap={3}>
              <ButtonPrimary
                onClick={() => onSubmit(validator)}
                disabled={updatePluginAttempt.status === 'processing'}
              >
                {isEditing ? 'Update' : 'Sync Okta Users'}
              </ButtonPrimary>
              <ButtonSecondary
                as={RouterLink}
                to={
                  isEditing
                    ? cfg.oss.getIntegrationStatusRoute('okta', 'okta')
                    : cfg.oss.getIntegrationEnrollRoute('okta')
                }
                disabled={updatePluginAttempt.status === 'processing'}
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

const requiredScopes = [
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

// TODO(kiosion): replace this w/ shared component for expandable section
const ExpandableWrapper = ({ children }: PropsWithChildren) => {
  const [expanded, setExpanded] = useState(false);
  return (
    <>
      <HelpButton
        label={expanded ? 'Collapse' : 'Expand'}
        onClick={() => setExpanded(v => !v)}
        icon={expanded ? ChevronUp : ChevronDown}
        mb={expanded ? 1 : 0}
      />
      {expanded && <Box ml={2}>{children}</Box>}
    </>
  );
};

const RequiredPermissionItem = ({
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

const GrantScopesStep = () => (
  <>
    <H2 mb={1}>Step 3: Grant Okta API Scopes</H2>
    <NumberedList>
      <ListItem>
        <Text>
          Go to the <b>Okta API Scopes</b> tab of your app and grant the
          following scopes:
        </Text>
        <ExpandableWrapper>
          <BulletList>
            {requiredScopes.map(([scope, desc]) => (
              <RequiredPermissionItem
                key={scope}
                text={scope}
                description={desc}
              />
            ))}
          </BulletList>
        </ExpandableWrapper>
      </ListItem>
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
        <ExpandableWrapper>
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
        </ExpandableWrapper>
      </ListItem>
      <ListItem mb={0}>
        <Text>
          Go to the <b>Admin Roles</b> tab of your app. Click{' '}
          <b>Edit Assignments</b>, and select the custom admin role you created,
          along with the resource set you created. Click <b>Save</b>.
        </Text>
      </ListItem>
    </NumberedList>
  </>
);

const OktaJwksStep = () => {
  const [showManualKeyInfo, setShowManualKeyInfo] = useState(false);
  const [oktaJwksAttempt, fetchOktaJwks] = useAsync(
    useCallback(
      () =>
        fetch(`${cfg.oss.baseUrl}/v1/.well-known/jwks-okta`)
          .then(res => res.json())
          .then(data => JSON.stringify(data.keys[0])),
      []
    )
  );

  useEffect(() => {
    if (showManualKeyInfo && !oktaJwksAttempt?.data) {
      void fetchOktaJwks();
    }
  }, [showManualKeyInfo, fetchOktaJwks]);

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
        {oktaJwksAttempt.status === 'processing' && (
          <Box textAlign="center" m={10}>
            <Indicator />
          </Box>
        )}
        {oktaJwksAttempt.status === 'success' && (
          <TextSelectCopy text={oktaJwksAttempt.data} bash={false} mb={2} />
        )}
        {oktaJwksAttempt.status === 'error' && (
          <>
            <Alert
              kind="danger"
              mb={0}
              primaryAction={{
                content: 'Retry',
                onClick: () => fetchOktaJwks(),
              }}
            >
              Failed to fetch key: {getErrMessage(oktaJwksAttempt.error)}
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
