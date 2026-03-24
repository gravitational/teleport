import { useState } from 'react';
import { Link as ReactRouterLink } from 'react-router';
import styled from 'styled-components';

import { Box, Flex, H2, Link, Text } from 'design';
import CardError from 'design/CardError';
import { FeatureName } from 'design/constants';
import * as Icons from 'design/Icon';
import { Mark } from 'design/Mark';
import { P } from 'design/Text/Text';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

import cfg from 'e-teleport/config';
import { OktaIntegrationSetUp } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/MultiStep/Okta/SetUp';
import { SCIMIntegrationSetUp } from 'e-teleport/Integrations/IntegrationEnroll/PluginEnroll/SCIM/SetUp';
import type {
  CloudHostablePlugin,
  SelfHostedPlugin,
} from 'e-teleport/services/plugins';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { UPGRADE_POLICY_URL } from 'teleport/services/sales';
import { CtaEvent } from 'teleport/services/userEvent';

import { AwsIdentityCenterPlugin } from './MultiStep/AwsIdentityCenter/Plugin';
import { CreateEmail } from './MultiStep/Email/CreateEmail';
import { EmailService } from './MultiStep/Email/EmailService';
import { FormMixin as EmailFormMixin } from './MultiStep/Email/FormMixin';
import {
  getSupportedEmailServices,
  supportedEmailServiceLabel,
} from './MultiStep/Email/types';
import { CreateEntra } from './MultiStep/Entra/CreateEntra';
import { FormMixin as EntraFormMixin } from './MultiStep/Entra/FormMixin';
import { RunScript } from './MultiStep/Entra/RunScript';
import { PluginEnrollSuccess } from './MultiStep/PluginEnrollSuccess';

export const plugins: (SelfHostedPlugin | CloudHostablePlugin)[] = [
  {
    type: 'slack',
    isOAuth: true,
    name: 'Slack',
    description: "Post access requests to your organization's Slack workspace.",
    icon: 'slack',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-slack/',
    fullName: 'Slack access request notifications',
    cloudHostable: true,
    selfHostable: false,
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          The Slack integration receives access requests from Teleport and posts
          them as Slack messages to alert reviewers.
        </P>
        <P>
          If an access request includes suggested reviewers, the Slack
          integration will add these to the list of channels to notify. If a
          suggested reviewer is an email address, the Slack integration will
          look up the direct message channel for that address and post a message
          in that channel. Otherwise, the integration will post messages in the
          default channel that you select on this screen.
        </P>
        <P>
          Please note that if you do not have permissions to add new apps to
          your Slack workspace, you will need to request approval in the next
          step. Once that approval is given, Slackbot will notify you within
          your workspace, and you will be able to connect Slack and Teleport by
          coming back to this view and clicking the “Connect Slack” button
          again.
        </P>
      </Text>
    ),
    Setup: () => (
      <Text>
        <ol>
          <li>
            Authenticate to the Slack workspace where you plan to manage access
            requests.
          </li>
          <li>
            Configure the default channel for the plugin at the bottom of this
            page.
          </li>
          <li>
            The integration will request permissions to access your workspace.
            Grant these permissions to the integration.
          </li>
        </ol>
      </Text>
    ),
    permissions: [
      {
        category: 'View content and info about your workspace',
        permissions: [
          { title: 'View people in your workspace' },
          {
            title: 'View email addresses of people in your workspace',
            description:
              'We will use email addresses to match Teleport users with their Slack profiles.',
          },
        ],
      },
      {
        category: 'Perform actions in channels & conversations',
        permissions: [
          { title: 'Send messages as @Teleport Cloud' },
          { title: 'Post messages to specific channels in Slack' },
        ],
      },
    ],
    FormMixin: () => {
      const [channel, setChannel] = useState('');
      return (
        <FieldInput
          width="260px"
          label="Default Channel"
          name="fallback_channel" // must be the same name as expected by the backend as form value
          rule={requiredField('Default channel must be specified')}
          value={channel}
          onChange={e => setChannel(e.target.value)}
          autoFocus
          placeholder="access-requests"
          toolTipContent="The default channel will receive all notifications about access requests. Request notifications will also be sent directly to assigned reviewers (if any)."
          icon={Icons.Hashtag}
        />
      );
    },
    NextSteps: ({ successData }) => {
      const fallbackChannel = successData?.slack?.fallback_channel;
      if (!fallbackChannel) {
        return <CardError>Failed to parse the response.</CardError>;
      }
      return (
        <P>
          As the final step, you should invite the "Teleport Cloud" application
          to channel <strong>{fallbackChannel}</strong> in your Slack workspace.
        </P>
      );
    },
  },
  {
    type: 'okta',
    name: 'Okta',
    description:
      'Setup SSO, user/group sync and enable access requests to Okta apps.',
    icon: 'okta',
    url: 'https://goteleport.com/docs/enroll-resources/application-access/okta/',
    cloudHostable: true,
    selfHostable: true,
    customSetup: true,
    fullName: 'Okta Integration',
    Setup: OktaIntegrationSetUp,
    tags: ['idp', 'scim'],
  },
  {
    type: 'scim',
    name: 'SCIM',
    icon: 'scim',
    description:
      'Allow different Identity Providers (IdPs) to push user and permission changes to Teleport.',

    url: 'https://goteleport.com/docs/enroll-resources/application-access/scim',
    cloudHostable: true,
    selfHostable: true,
    customSetup: true,
    fullName: 'SCIM Integration',
    Setup: SCIMIntegrationSetUp,
    tags: ['idp', 'scim'],
  },
  {
    type: 'opsgenie',
    description: 'Enable access notifications based on your support schedule.',
    name: 'Opsgenie',
    icon: 'opsgenie',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-requests/resource-requests/', // TODO(lisa): change to opsgenie docs (wip)
    cloudHostable: true,
    selfHostable: true,
    fullName: 'Opsgenie access request notifications',
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          Integrating with Opsgenie allows Teleport access requests to show up
          as alerts in the specified Opsgenie schedule.
        </P>
        <P>
          You will need to provide an API key with the following permissions
          “Read Access” and “Create and Update Access”.
        </P>
      </Text>
    ),
    Setup: () => (
      <Text>
        <P>
          Generate an API key that the Opsgenie plugin will use to create and
          modify alerts as well as list users, services, and on-call policies.
        </P>

        <ol>
          <li>
            Follow the instructions in the{' '}
            <Link
              target="_blank"
              href="https://support.atlassian.com/opsgenie/docs/api-key-management/"
            >
              Opsgenie documentation
            </Link>
            , assigning the following permissions to your API key:
            <ul>
              <li>read</li>
              <li>create</li>
              <li>update</li>
            </ul>
          </li>
          <li>
            Copy the API key so you can paste it in the form on this screen.
          </li>
        </ol>
      </Text>
    ),
    permissions: [
      {
        category: 'Creating Alerts',
        permissions: [
          {
            title: 'Create alerts for incoming access requests',
          },
        ],
      },
    ],
    FormMixin: () => {
      const [apiEndpoint, setApiEndpoint] = useState<Option>();
      const [token, setToken] = useState('');
      const [scheduleName, setScheduleName] = useState('');
      return (
        <>
          <FieldSelect
            width="250px"
            label="API Endpoint"
            name="apiEndpoint" // must be the same name as expected by the backend as form value
            rule={requiredField<Option>('API Endpoint Required')}
            value={apiEndpoint}
            onChange={o => setApiEndpoint(o as Option)}
            autoFocus
            options={[
              {
                value: 'https://api.opsgenie.com',
                label: 'https://api.opsgenie.com',
              },
              {
                value: 'https://api.eu.opsgenie.com',
                label: 'https://api.eu.opsgenie.com',
              },
            ]}
            placeholder="Select a API Endpoint"
            isSearchable
            mb={3}
          />
          <FieldInput
            width="500px"
            label="API Key"
            name="apiKey" // must be the same name as expected by the backend as form value
            rule={requiredField('API Key Required')}
            value={token}
            type="password"
            onChange={e => setToken(e.target.value)}
            placeholder="abc-def...-123"
            toolTipContent="API Key is used to request to Opsgenie REST API and requires the permissions: 'Read Access' and 'Create and Update Access'"
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Default Schedule Name (Optional)"
            name="scheduleName" // must be the same name as expected by the backend as form value
            value={scheduleName}
            onChange={e => setScheduleName(e.target.value)}
            placeholder="schedule name"
            toolTipContent="The name of schedule that will receive alerts"
          />
        </>
      );
    },
    NextSteps: () => {
      return null; // TODO(lisa): help with next step blurb, empty for now
    },
  },
  {
    type: 'jamf',
    name: 'Jamf',
    icon: 'jamf',
    description:
      'Allow trusted devices in Teleport to match available devices in your Jamf inventory.',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/device-trust/jamf-integration/',
    cloudHostable: true,
    selfHostable: false,
    disabledIfNoMdmSupport: true,
    fullName: 'Jamf Integration for Device Trust',
    tags: ['devicetrust'],
    Description: () => (
      <Text>
        <P>
          Jamf integration updates trusted devices in Teleport to match
          available devices in your Jamf inventory. For more details, see our
          docs page about{' '}
          <Link
            href="https://goteleport.com/docs/admin-guides/access-controls/device-trust/jamf-integration/"
            target="_blank"
          >
            Device Trust and the Jamf integration
          </Link>
          .
        </P>
      </Text>
    ),
    Setup: () => (
      <Text>
        <ol>
          <li>
            Create a Jamf role with the "Read Computers" privilege. Follow the
            instructions at{' '}
            <Link
              href="https://learn.jamf.com/en-US/bundle/jamf-pro-documentation-current/page/API_Roles_and_Clients.html"
              target="_blank"
            >
              Jamf API Roles and Clients.
            </Link>
          </li>
          <li>
            Create a Jamf API client, assigning to it the role created above.
          </li>
          <li>
            Take note of the Client ID and Client Secret of your API client to
            configure the Jamf integration on this screen.
          </li>
        </ol>
      </Text>
    ),
    permissions: [
      {
        category: 'Jamf API',
        permissions: [
          {
            title: 'Read Computers',
            description:
              'A Jamf role with the Read Computers permission must be assigned to the API client.',
          },
        ],
      },
      {
        category: 'Device Registration, Enrollment and Deletion',
        permissions: [
          {
            title:
              'Synchronize devices from Jamf to Teleport device inventory.',
          },
        ],
      },
    ],
    FormMixin: () => {
      const [clientId, setClientId] = useState('');
      const [clientSecret, setClientSecret] = useState('');
      const [apiEndpoint, setApiEndpoint] = useState('');
      return (
        <>
          <FieldInput
            width="500px"
            label="Jamf API Endpoint"
            name="apiEndpoint" // must be the same name as expected by the backend as form value
            rule={requiredField('API endpoint must be specified')}
            value={apiEndpoint}
            onChange={e => setApiEndpoint(e.target.value)}
            autoFocus
            placeholder="https://yourserver.jamfcloud.com"
            toolTipContent="URL of Jamf API. (e.g. https://yourtenant.jamfcloud.com)"
          />
          <FieldInput
            width="500px"
            label="Jamf API Client ID"
            name="clientId" // must be the same name as expected by the backend as form value
            rule={requiredField('Client ID must be specified')}
            value={clientId}
            onChange={e => setClientId(e.target.value)}
            placeholder="Client ID"
            toolTipContent="Jamf API Client ID."
          />
          <FieldInput
            width="500px"
            label="Jamf API Client Secret"
            name="clientSecret" // must be the same name as expected by the backend as form value
            rule={requiredField('Client Secret must be specified')}
            value={clientSecret}
            onChange={e => setClientSecret(e.target.value)}
            placeholder="Client Secret"
            type="password"
            toolTipContent="Jamf API Client Secret."
          />
        </>
      );
    },
    NextSteps: () => {
      return (
        <P>
          Jamf integration is configured for your cluster. Depending on the size
          of your Jamf inventory, it may take a few minutes to sync with{' '}
          <ReactRouterLink to={cfg.routes.deviceTrust}>
            Trusted Devices
          </ReactRouterLink>{' '}
          in Teleport.
        </P>
      );
    },
  },
  {
    type: 'intune',
    name: 'Microsoft Intune',
    description:
      'Update trusted devices in Teleport to match available devices in your Intune inventory.',
    icon: 'intune',
    url: 'https://goteleport.com/docs/identity-governance/device-trust/intune-integration/',
    cloudHostable: true,
    selfHostable: true,
    disabledIfNoMdmSupport: true,
    fullName: 'Microsoft Intune Integration for Device Trust',
    tags: ['devicetrust'],
    Description: () => (
      <Text>
        <P>
          Microsoft Intune integration updates trusted devices in Teleport to
          match available devices in your Intune inventory. For more details,
          see our docs page about{' '}
          <Link
            href="https://goteleport.com/docs/identity-governance/device-trust/intune-integration/"
            target="_blank"
          >
            Device Trust and the Intune integration
          </Link>
          .
        </P>
      </Text>
    ),
    permissions: [
      {
        category: 'Microsoft Graph',
        permissions: [
          {
            title: 'DeviceManagementManagedDevices.Read.All',
            description:
              'An application permission that lets the integration pull devices from Intune into Teleport.',
          },
        ],
      },
    ],
    Setup: () => (
      <Text>
        <ol>
          <li>
            Provide{' '}
            <Link
              href="https://learn.microsoft.com/en-us/partner-center/account-settings/find-ids-and-domain-names#find-the-microsoft-entra-tenant-id-and-primary-domain-name"
              target="_blank"
            >
              the primary domain or the Microsoft Entra tenant ID
            </Link>{' '}
            in the form below.
          </li>
          <li>
            <Link
              href="https://learn.microsoft.com/en-us/graph/auth-register-app-v2"
              target="_blank"
            >
              Register an application
            </Link>{' '}
            in the Microsoft identity platform.
            <ul>
              <li>
                Select "Accounts in this organizational directory only" as the
                supported account types.
              </li>
              <li>Skip the Redirect URI section and click "Register".</li>
              <li>
                Copy "Application (client) ID" and paste it the form below.
              </li>
            </ul>
          </li>
          <li>
            <Link
              href="https://learn.microsoft.com/en-us/entra/identity-platform/quickstart-configure-app-access-web-apis#application-permission-to-microsoft-graph"
              target="_blank"
            >
              Add application permission
            </Link>{' '}
            for Microsoft Graph's{' '}
            <code>DeviceManagementManagedDevices.Read.All</code>.
            <ul>
              <li>
                Make sure the permission is granted by an administrator.
                Otherwise the Graph API will return an error.
              </li>
            </ul>
          </li>
          <li>
            <Link
              href="https://learn.microsoft.com/en-us/graph/auth-register-app-v2#option-2-add-a-client-secret"
              target="_blank"
            >
              Add a client secret
            </Link>
            . Copy the resulting value and paste it in the form below.
          </li>
        </ol>
      </Text>
    ),
    FormMixin: () => {
      const [tenant, setTenant] = useState('');
      const [clientId, setClientId] = useState('');
      const [clientSecret, setClientSecret] = useState('');
      return (
        <>
          <FieldInput
            width="500px"
            label="Primary domain or Microsoft Entra tenant ID"
            name="tenant" // must be the same name as expected by the backend as form value
            rule={requiredField('Tenant must be specified')}
            value={tenant}
            onChange={e => setTenant(e.target.value)}
            autoFocus
            placeholder="contoso.onmicrosoft.com"
          />
          <FieldInput
            width="500px"
            label="Application (client) ID"
            name="clientId" // must be the same name as expected by the backend as form value
            rule={requiredField('Application (client) ID must be specified')}
            value={clientId}
            onChange={e => setClientId(e.target.value)}
            placeholder="9bbf1ecc-1aba-4293-8465-47e511dae942"
          />
          <FieldInput
            width="500px"
            label="Client secret value"
            name="clientSecret" // must be the same name as expected by the backend as form value
            rule={requiredField('Client secret must be specified')}
            value={clientSecret}
            onChange={e => setClientSecret(e.target.value)}
            type="password"
          />
        </>
      );
    },
    NextSteps: () => {
      return (
        <P>
          The Intune integration is configured for your cluster. Depending on
          the size of your Intune inventory, it may take a few minutes to sync
          with{' '}
          <ReactRouterLink to={cfg.routes.deviceTrust}>
            Trusted Devices
          </ReactRouterLink>{' '}
          in Teleport.
        </P>
      );
    },
  },
  {
    type: 'servicenow',
    name: 'ServiceNow',
    description: 'Create ServiceNow incidents from Teleport access requests.',
    icon: 'servicenow',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-requests/resource-requests/',
    cloudHostable: true,
    selfHostable: true,
    fullName: 'ServiceNow Integration',
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          ServiceNow plugin creates ServiceNow incidents for Teleport access
          requests.
        </P>
      </Text>
    ),
    permissions: [
      {
        category: 'Creating Alerts',
        permissions: [
          {
            title: 'Read and write access to ServiceNow API.',
            description:
              'Teleport will authenticate to ServiceNow API using ServiceNow account credential (username + password).',
          },
        ],
      },
    ],
    Setup: () => (
      <Text>
        <P>
          The ServiceNow integration requires a ServiceNow user with permissions
          to read from and write to the incident table. This requires a role
          with the “sn_incident_read” and “sn_incident_write” roles.
        </P>
        <ol>
          <li>
            Ensure that your ServiceNow account includes the{' '}
            <Link
              target="_blank"
              href="https://www.servicenow.com/docs/r/it-service-management/incident-management/req-itsm-roles-inci-mgmt.html"
            >
              ITSM Roles plugin
            </Link>
            , which enables the “sn_incident_read” and “sn_incident_write”
            roles.
          </li>
          <li>
            <Link
              target="_blank"
              href="https://www.servicenow.com/docs/r/platform-administration/user-administration/t_CreateARole.html"
            >
              Create a ServiceNow role
            </Link>{' '}
            that we will later assign to the user account for the Teleport
            integration.
          </li>
          <li>
            <Link
              target="_blank"
              href="https://www.servicenow.com/docs/r/platform-administration/user-administration/t_AddARoleToAnExistingRole.html"
            >
              Edit the role you created
            </Link>{' '}
            to add the “sn_incident_read” and “sn_incident_write” roles.
          </li>
          <li>
            Follow the{' '}
            <Link
              target="_blank"
              href="https://www.servicenow.com/docs/r/platform-administration/user-administration/t_CreateAUser.html"
            >
              ServiceNow documentation
            </Link>{' '}
            to create a ServiceNow user. Paste the name and password of the user
            in the form at the bottom of this page.
          </li>
          <li>
            <Link
              target="_blank"
              href="https://www.servicenow.com/docs/r/platform-administration/user-administration/t_AssignARoleToAUser.html"
            >
              Assign the role you created
            </Link>{' '}
            to the ServiceNow user you created.
          </li>
        </ol>
      </Text>
    ),
    FormMixin: () => {
      const [username, setUsername] = useState('');
      const [password, setPassword] = useState('');
      const [apiEndpoint, setApiEndpoint] = useState('');
      const [closeCode, setCloseCode] = useState('');
      return (
        <>
          <FieldInput
            width="500px"
            label="ServiceNow API Endpoint"
            name="apiEndpoint" // must be the same name as expected by the backend as form value
            rule={requiredField('API endpoint must be specified')}
            value={apiEndpoint}
            onChange={e => setApiEndpoint(e.target.value)}
            autoFocus
            placeholder="https://yourserver.servicenowcloud.com"
            toolTipContent="URL of ServiceNow API. (e.g. https://example-servicenow-instance.com)"
          />
          <FieldInput
            width="500px"
            label="ServiceNow Account Username"
            name="username" // must be the same name as expected by the backend as form value
            rule={requiredField('Username must be specified')}
            value={username}
            onChange={e => setUsername(e.target.value)}
            placeholder="Username"
            toolTipContent="Username of the account that will be used to authenticate with Servicenow API."
          />
          <FieldInput
            width="500px"
            label="ServiceNow Account Password"
            name="password" // must be the same name as expected by the backend as form value
            rule={requiredField('Password must be specified')}
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder="Password"
            type="password"
            toolTipContent="Password of the account that will be used to authenticate with ServiceNow API."
          />
          <FieldInput
            width="500px"
            label="ServiceNow Close Code"
            name="closeCode" // must be the same name as expected by the backend as form value
            rule={requiredField('Close code must be specified')}
            value={closeCode}
            onChange={e => setCloseCode(e.target.value)}
            autoFocus
            placeholder="Resolved"
            toolTipContent="ServiceNow Close code to resolve incidents with."
          />
        </>
      );
    },
    NextSteps: () => {
      return null;
    },
  },
  {
    type: 'jira',
    name: 'Jira',
    description: 'Manage access requests through Jira incidents.',
    fullName: 'Jira access request management',
    icon: 'jira',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-jira',
    cloudHostable: true,
    selfHostable: true,
    tags: ['notifications'],
    permissions: [
      {
        category: 'Read/Write on Jira Issues',
        permissions: [
          {
            title: 'Read',
            description: 'Monitor Teleport-created Issues',
          },
          {
            title: 'Write',
            description: 'Create, Comment on, and Resolve Issues',
          },
        ],
      },
    ],
    Description: () => (
      <Text>
        <P>
          The Teleport Jira integration allows you to manage Teleport access
          requests using Jira tickets.
        </P>
      </Text>
    ),
    Setup: () => (
      <Text>
        <ol>
          <li>
            Follow the{' '}
            <Link
              target="_blank"
              href="https://support.atlassian.com/jira-software-cloud/docs/create-a-new-project/"
            >
              Jira documentation
            </Link>{' '}
            to create a project. Ensure that the project has the following
            attributes:
            <ul>
              <li>
                Uses the{' '}
                <Link
                  target="_blank"
                  href="https://www.atlassian.com/software/jira/templates/kanban"
                >
                  Kanban
                </Link>{' '}
                template.
              </li>
              <li>Is a company-managed project.</li>
            </ul>
          </li>
          <li>
            Edit the statuses in your board so it contains the following four:
            <ul>
              <li>Pending</li>
              <li>Approved</li>
              <li>Denied</li>
              <li>Expired</li>
            </ul>
          </li>

          <li>
            Create a column with the same name as each status. If your project
            board does not contain these (and only these) columns, each with a
            status of the same name, the Jira access request integration will
            behave in unexpected ways. Remove all other columns and statuses.
          </li>
          <li>
            The Teleport Jira integration expects tasks in your project board to
            include a field called “teleportAccessRequestId”, which it uses to
            track individual access requests. This prevents users from tampering
            with or forging access requests. Follow the{' '}
            <Link
              target="_blank"
              href="https://support.atlassian.com/jira-cloud-administration/docs/create-a-custom-field/"
            >
              Jira documentation
            </Link>{' '}
            to create a custom field and add it to the project you created.
            Ensure that the custom field has the following attributes:
            <ul>
              <li>The name of the field must be “teleportAccessRequestId”.</li>
              <li>The field must be a short text field.</li>
            </ul>
          </li>

          <li>
            Obtain an API token for the Teleport Jira integration to use to make
            changes to your Jira project by following the{' '}
            <Link
              target="_blank"
              href="https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/#Create-an-API-token"
            >
              Jira documentation
            </Link>
            . Copy the API key into the form on this screen.{' '}
          </li>
        </ol>
      </Text>
    ),
    FormMixin: () => {
      const [username, setUsername] = useState('');
      const [token, setToken] = useState('');
      const [addr, setAddr] = useState('');
      const [project, setProject] = useState('');
      const [issueType, setIssueType] = useState('');

      return (
        <>
          <FieldInput
            width="500px"
            label="Jira Server Address"
            name="addr" // must be the same name as expected by the backend as form value
            rule={requiredField('Jira server address required')}
            value={addr}
            onChange={e => setAddr(e.target.value)}
            placeholder="https://myserver.jira.com"
            toolTipContent="Address of the Jira server to enroll"
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Jira Username"
            name="username" // must be the same name as expected by the backend as form value
            rule={requiredField('Jira Username Required')}
            value={username}
            onChange={e => setUsername(e.target.value)}
            placeholder="Jira Username"
            toolTipContent="The Jira username that Teleport will use when logging in to Jira."
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Jira API Key"
            name="apiKey" // must be the same name as expected by the backend as form value
            rule={requiredField('Jira API Key Required')}
            value={token}
            type="password"
            onChange={e => setToken(e.target.value)}
            placeholder="AAA...-123"
            toolTipContent="API Key generated for the user specified in Jira Username"
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Jira Project Key"
            name="project" // must be the same name as expected by the backend as form value
            rule={requiredField('Jira Project Key Required')}
            value={project}
            onChange={e => setProject(e.target.value)}
            placeholder="Jira Project Key"
            toolTipContent="This is a small (usually 3 or 4 character) identifier that Jira adds to the start of issue numbers that identifies the project they belong to."
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Jira Issue Type"
            name="issueType" // must be the same name as expected by the backend as form value
            rule={requiredField('Jira Issue Type Required')}
            value={issueType}
            onChange={e => setIssueType(e.target.value)}
            placeholder="Task"
            toolTipContent="The Jira issue type to create."
            mb={3}
          />
        </>
      );
    },
    NextSteps: () => {
      return (
        <Text>
          <P>
            Teleport will create issues in your Jira project in response to
            access requests.
          </P>
          <P>
            Adding the <em>Pending</em>, <em>Approved</em> and <em>Denied</em>{' '}
            columns to your Jira project board will also allow Teleport to
            automatically update the status of these Jira issues as access
            requests are approved or denied. For more information, consult the{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-jira/#step-36-set-up-your-jira-project"
            >
              Set up your Jira project
            </Link>{' '}
            section of Teleport's Jira guide.
          </P>
        </Text>
      );
    },
  },
  {
    type: 'pagerduty',
    name: 'PagerDuty',
    description: 'Manage access requests through PagerDuty incidents',
    icon: 'pagerduty',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-pagerduty/',
    fullName: 'PagerDuty access request management',
    cloudHostable: true,
    selfHostable: true,
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          The Teleport integration with PagerDuty allows your team to treat
          Teleport permission requests as Pagerduty incidents and provides
          Pagerduty special actions to approve or deny permission requests.
        </P>
      </Text>
    ),
    Setup: () => (
      <Text>
        <P>
          You will need to generate an API key for the PagerDuty integration to
          use to create and modify incidents as well as list users, services,
          and on-call policies.
        </P>

        <ol>
          <li>
            Follow the{' '}
            <Link
              target="_blank"
              href="https://support.pagerduty.com/docs/api-access-keys#generate-a-general-access-rest-api-key"
            >
              PagerDuty documentation
            </Link>{' '}
            to create a REST API key. The key <strong>must not</strong> be read
            only.
          </li>
          <li>
            Copy the key and paste it into the “PagerDuty API Key” field on this
            screen.
          </li>
        </ol>
      </Text>
    ),
    permissions: [
      {
        category: 'Users',
        permissions: [
          {
            title: 'Read',
            description: 'Query user IDs to determine on-call user.',
          },
        ],
      },
      {
        category: 'Incidents',
        permissions: [
          {
            title: 'Read',
            description: 'Monitor Teleport-created Incidents',
          },
          {
            title: 'Write',
            description: 'Create, Comment on, and Resolve Incidents',
          },
        ],
      },
      {
        category: 'On-Call Schedules',
        permissions: [
          {
            title: 'Read',
            description: 'Reads On-Call schedules to determine on-call user.',
          },
        ],
      },
    ],
    FormMixin: () => {
      const [token, setToken] = useState('');
      const [email, setEmail] = useState('');
      return (
        <>
          <FieldInput
            width="500px"
            label="PagerDuty User Email"
            name="email" // must be the same name as expected by the backend as form value
            rule={requiredField('PagerDuty User Email Required')}
            value={email}
            onChange={e => setEmail(e.target.value)}
            placeholder="root@example.com"
            toolTipContent="Email address of PagerDuty user to sign incidents"
            mb={3}
          />
          <FieldInput
            width="500px"
            label="PagerDuty API Key"
            name="apiKey" // must be the same name as expected by the backend as form value
            rule={requiredField('API Key Required')}
            value={token}
            type="password"
            onChange={e => setToken(e.target.value)}
            placeholder="abc-def...-123"
            toolTipContent="API Key is used to access the PagerDuty REST API"
            mb={3}
          />
        </>
      );
    },
    NextSteps: () => {
      return (
        <Text>
          <P>
            Be sure to configure the “pagerduty_notify_service” and
            “pagerduty_services” annotations in the Teleport roles you want
            PagerDuty to manage.
          </P>
          <P>
            For more information, consult the <em>Define RBAC Resources</em>{' '}
            section of the Teleport{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-pagerduty/#step-28-define-rbac-resources"
            >
              Access Requests with PagerDuty
            </Link>{' '}
            guide.
          </P>
        </Text>
      );
    },
  },
  {
    type: 'email',
    name: 'Email',
    description:
      'Email access requests to specified addresses or distribution lists',
    icon: 'email',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-email/',
    fullName: 'Email Integration',
    cloudHostable: true,
    selfHostable: true,
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          The Teleport integration with Email allows you to send Just-in-Time
          Access Request notifications to users via email.
        </P>
      </Text>
    ),
    Setup: () => {
      const supportedServices = getSupportedEmailServices();

      return (
        <Text>
          <ol>
            <li>
              Configure the desired sender email and default fallback recipient.
            </li>
            {supportedServices.length == 1 ? (
              <li>
                Currently the only supported service is{' '}
                <strong>
                  {supportedEmailServiceLabel(supportedServices[0])}
                </strong>
                . Click next for further configuration.
              </li>
            ) : (
              <li>
                Currently supported services are{' '}
                {new Intl.ListFormat('en-US').format(
                  supportedServices.map(supportedEmailServiceLabel)
                )}
                . Select the desired email service and click next for further
                configuration.
              </li>
            )}
          </ol>
        </Text>
      );
    },
    views: () => [
      { title: 'Connect Email', component: CreateEmail },
      { title: 'Set up Email Service', component: EmailService },
      { title: 'Finished', component: PluginEnrollSuccess, hide: true },
    ],
    FormMixin: EmailFormMixin,
    NextSteps: () => {
      return (
        <Text>
          <P>
            For help with configuring access request notification routing rules,
            consult the{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/notification-routing-rules"
            >
              Notification Routing Rules
            </Link>{' '}
            section of Teleport's documentation.
          </P>
        </Text>
      );
    },
  },
  {
    type: 'discord',
    name: 'Discord',
    description: 'Post access requests to channels on your Discord server.',
    fullName: 'Discord access request notifications',
    icon: 'discord',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-discord/',
    cloudHostable: true,
    selfHostable: false,
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          The Discord integration receives access request events from the
          Teleport Auth Service, formats them into Discord messages, and sends
          them to the Discord API to post them in your guild (Discord server).
        </P>
      </Text>
    ),
    Setup: () => (
      <ol>
        <li>
          Follow the{' '}
          <Link
            target="_blank"
            href="https://docs.discord.com/developers/quick-start/getting-started"
          >
            Discord documentation
          </Link>{' '}
          to create a bot application and install it on your Discord server. The
          application must have the following attributes:
          <ul>
            <li>It must not be a public bot.</li>
            <li>It must have the “bot” and “Send Messages” permissions.</li>
          </ul>
        </li>
        <li>
          After creating a Discord application, retrieve its API token and paste
          it in the form on this screen.
        </li>
      </ol>
    ),
    FormMixin: () => {
      const [token, setToken] = useState('');
      const [channels, setChannels] = useState('');

      return (
        <>
          <FieldInput
            width="500px"
            label="Discord API Token"
            name="token" // must be the same name as expected by the backend as form value
            rule={requiredField('API Token Required')}
            value={token}
            onChange={e => setToken(e.target.value)}
            placeholder="ABCD..."
            toolTipContent="Discord Bot API token"
            type="password"
            mb={3}
          />

          <FieldInput
            width="500px"
            label="Channel(s)"
            name="channels" // must be the same name as expected by the backend as form value
            rule={requiredField('Channel is required')}
            value={channels}
            onChange={e => setChannels(e.target.value)}
            placeholder="123456789012345678"
            toolTipContent="A comma-separated list of Discord channel IDs"
            mb={3}
          />
        </>
      );
    },
  },
  {
    type: 'mattermost',
    name: 'Mattermost',
    description:
      'Post Teleport access requests as Mattermost messages to alert reviewers.',
    icon: 'mattermost',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-mattermost/',
    fullName: 'Mattermost access request notifications',
    cloudHostable: true,
    selfHostable: true,
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          The Mattermost integration receives access requests from Teleport and
          posts them as Mattermost messages to alert reviewers.
        </P>

        <P>
          If an access request includes suggested reviewers, the Mattermost
          integration will add these to the list of channels to notify. If a
          suggested reviewer is an email address, the integration will look up
          the direct message channel for that address and post a message in that
          channel. Otherwise, the integration will post the message to the
          default channel that you specify on this screen.
        </P>
      </Text>
    ),
    Setup: () => (
      <Text>
        <ol>
          <li>
            Follow the Mattermost{' '}
            <Link
              target="_blank"
              href="https://developers.mattermost.com/integrate/reference/bot-accounts/#user-interface-ui"
            >
              documentation
            </Link>{' '}
            to create a bot account. Ensure that the bot account has the
            following attributes:
            <ul>
              <li>The role must be “Member”.</li>
              <li>“post:all” must be set to “Enabled”.</li>
            </ul>
          </li>
          <li>
            After creating the bot account, use the resulting OAuth 2.0 token
            when you configure the Mattermost integration on this screen.
          </li>
        </ol>
      </Text>
    ),
    FormMixin: () => {
      const [url, setUrl] = useState('');
      const [token, setToken] = useState('');
      const [team, setTeam] = useState('');
      const [channel, setChannel] = useState('');
      const [email, setEmail] = useState('');

      const teamChannelRequired = `${team}${channel}`.length > 0;
      return (
        <>
          <FieldInput
            width="500px"
            label="Mattermost Server URL"
            name="url" // must be the same name as expected by the backend as form value
            rule={requiredField('Mattermost Server URL Required')}
            value={url}
            onChange={e => setUrl(e.target.value)}
            placeholder="https://example.mattermost.com"
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Mattermost Bot Access Token"
            name="token" // must be the same name as expected by the backend as form value
            rule={requiredField('Bot Access Token Required')}
            value={token}
            type="password"
            onChange={e => setToken(e.target.value)}
            placeholder="token"
            toolTipContent={`Not to be confused with the token ID. The \
            integration requires the access token that was generated \
            when you created your bot.`}
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Mattermost User Email (Optional)"
            name="email" // must be the same name as expected by the backend as form value
            value={email}
            onChange={e => setEmail(e.target.value)}
            placeholder="email"
            toolTipContent={`The email address of a Mattermost user to notify via \
             a direct message when the integration receives an access request event`}
            mb={3}
          />
          <Box width="500px">
            <Text mb={1}>Mattermost Team and Channel (Optional):</Text>
            <Flex justifyContent="space-between">
              <FieldInput
                width="240px"
                label="Team Name"
                name="team" // must be the same name as expected by the backend as form value
                rule={
                  teamChannelRequired
                    ? requiredField('Team Name Required')
                    : undefined
                }
                value={team}
                onChange={e => setTeam(e.target.value)}
                placeholder="team-name"
                toolTipContent="The name of your Mattermost workspace"
                mr={2}
              />
              <FieldInput
                width="240px"
                label="Channel Name"
                name="channel" // must be the same name as expected by the backend as form value
                rule={
                  teamChannelRequired
                    ? requiredField('Channel Name Required')
                    : undefined
                }
                value={channel}
                onChange={e => setChannel(e.target.value)}
                placeholder="channel-name"
                toolTipContent="Name of the channel to post access requests to"
              />
            </Flex>
          </Box>
        </>
      );
    },
    NextSteps: () => {
      return (
        <Text>
          <P>
            For help with configuring roles for access requests, consult the{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-mattermost/#step-18-define-rbac-resources"
            >
              Define RBAC Resources
            </Link>{' '}
            section of Teleport's Mattermost guide.
          </P>
        </Text>
      );
    },
  },
  {
    type: 'msteams',
    name: 'Microsoft Teams',
    description:
      'Send notifications on Microsoft Teams for incoming Teleport access requests.',
    icon: 'microsoftteams',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/ssh-approval-msteams/',
    cloudHostable: true,
    selfHostable: true,
    fullName: 'Microsoft Teams access request notifications',
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          Integrating with Microsoft Teams allows Teleport to send notifications
          via Microsoft Teams about incoming access requests. For more details,
          see our docs page about{' '}
          <Link
            href="https://goteleport.com/docs/identity-governance/access-requests/plugins/msteams/"
            target="_blank"
          >
            Access Requests with Microsoft Teams
          </Link>
          .
        </P>
      </Text>
    ),
    permissions: [
      {
        category: 'Permissions required by the Microsoft Azure App.',
        permissions: [
          {
            title: 'AppCatalog.Read.All',
            description:
              'Used to list Teams Apps and check the app is installed.',
          },
          {
            title: 'User.Read.All',
            description: 'Used to get notification recipients.',
          },
          {
            title: 'TeamsAppInstallation.ReadWriteSelfForUser.All',
            description:
              'Used to initiate communication with a user that never interacted with the Teams App before.',
          },
          {
            title: 'TeamsAppInstallation.ReadWriteSelfForTeam.All',
            description:
              'Used to discover if the app is installed in the Team.',
          },
        ],
      },
    ],
    FormMixin: () => {
      const [token, setToken] = useState('');
      const [appID, setAppID] = useState('');
      const [tenantID, setTenantID] = useState('');
      const [teamsAppID, setTeamsAppID] = useState('');
      const [region, setRegion] = useState('');
      const [defaultRecipient, setDefaultRecipient] = useState('');
      return (
        <>
          <FieldInput
            width="500px"
            label="App ID"
            name="appID" // must be the same name as expected by the backend as form value
            rule={requiredField('App ID Required')}
            value={appID}
            onChange={e => setAppID(e.target.value)}
            placeholder="App ID"
            toolTipContent={`The Azure Bot's App ID (Microsoft App ID) \
              can be found on its "Configuration" page.`}
          />
          <FieldInput
            width="500px"
            label="Tenant ID"
            name="tenantID" // must be the same name as expected by the backend as form value
            rule={requiredField('Tenant ID Required')}
            value={tenantID}
            onChange={e => setTenantID(e.target.value)}
            placeholder="Tenant ID"
            toolTipContent={`The Azure Bot's Tenant ID (App Tenant ID) \
              can be found on its "Configuration" page.`}
          />
          <FieldInput
            width="500px"
            label="TeamsApp ID"
            name="teamsAppID" // must be the same name as expected by the backend as form value
            value={teamsAppID}
            onChange={e => setTeamsAppID(e.target.value)}
            placeholder="Teams App ID "
            toolTipContent={`The Teams App ID (External app ID) \
              can be found in the Microsoft Teams admin center by navigating to \
              the "Manage apps" page and viewing your app's details.`}
          />
          <FieldInput
            width="500px"
            label="Region"
            name="region" // must be the same name as expected by the backend as form value
            value={region}
            onChange={e => setRegion(e.target.value)}
            placeholder="Region"
            toolTipContent="Deprecated: API region should no longer be specified."
          />
          <FieldInput
            width="500px"
            label="App secret"
            name="appSecret" // must be the same name as expected by the backend as form value
            rule={requiredField('App secret Required')}
            value={token}
            type="password"
            onChange={e => setToken(e.target.value)}
            placeholder="abc-def...-123"
            toolTipContent={`The Azure Bot's App secret can be configured by \
              navigating to its "Configuration" page, selecting "Manage Password", \
              and creating a client secret.`}
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Default Recipient"
            name="defaultRecipient" // must be the same name as expected by the backend as form value
            rule={requiredField('Default recipient Required')}
            value={defaultRecipient}
            onChange={e => setDefaultRecipient(e.target.value)}
            placeholder="https://teams.microsoft.com/l/channel/..."
            toolTipContent={`The default recipient for access request notifications \
              must be a Teams user's email or a Teams channel URL, which can be \
              obtained by opening the channel and selecting "Copy link".`}
          />
        </>
      );
    },
    NextSteps: () => {
      return null;
    },
  },
  {
    type: 'entra-id',
    name: 'Microsoft Entra ID',
    description:
      'Synchronize users and groups from your Entra ID directory to Teleport',
    icon: 'entraid',
    url: '',
    fullName: 'Entra ID directory synchronization',
    cloudHostable: true,
    selfHostable: true,
    // todo (michellescripts) replace with new entitlement `EntraIDSync`
    requiresIgs: true,
    tags: ['idp'],
    permissions: [
      {
        category: 'Read information about your Entra ID tenant',
        permissions: [
          { title: 'Users, groups and roles in the directory' },
          { title: 'Connected enterprise applications' },
        ],
      },
      {
        category: 'Create an enterprise application for your Teleport cluster',
        permissions: [
          { title: 'Provide SSO to your Teleport cluster via SAML' },
          { title: 'Allow access to your Entra ID tenant data via OIDC' },
        ],
      },
    ],
    Description: () => (
      <Box mb={3}>
        <Text>
          The Entra ID integration synchronizes users and groups from your Entra
          ID directory and provides an SSO connector to sign into Teleport via
          Entra ID.
        </Text>
        <H2 mt={3}>Included with {FeatureName.IdentityGovernance}:</H2>
        <StyledUl>
          <li>
            <strong>Directory synchronization</strong>: Routinely synchronizes
            Entra ID users and groups with Teleport
          </li>
          <li>
            <strong>SSO integration</strong>: A SAML SSO connector that grants
            Entra ID directory users access to the Teleport cluster.
          </li>
        </StyledUl>
        <H2 mt={3}>Included with {FeatureName.IdentitySecurity}:</H2>
        <StyledUl>
          <li>
            <strong>Access Graph integration</strong>: analyze your Entra ID
            directory and SSO applications using Teleport Access Graph.
          </li>
        </StyledUl>
        {(!cfg.oss.entitlements.Policy.enabled || !cfg.oss.isPolicyEnabled) && (
          <ButtonLockedFeature
            event={CtaEvent.CTA_ENTRA_ID}
            width={'auto'}
            mt={2}
            mb={3}
            url={UPGRADE_POLICY_URL}
          >
            Unlock Access Graph integration with {FeatureName.IdentitySecurity}
          </ButtonLockedFeature>
        )}
      </Box>
    ),

    NextSteps: () => {
      return (
        <>
          <P>
            To assign roles based on SSO attributes visit the{' '}
            <ReactRouterLink to={cfg.oss.routes.sso}>
              Auth Connectors
            </ReactRouterLink>{' '}
            page.
          </P>
          <P>
            It may take a while before all users and groups are synced to
            Teleport.
          </P>
        </>
      );
    },

    views: () => [
      { title: 'Connect Entra', component: CreateEntra },
      { title: 'Set up permissions', component: RunScript },
      { title: 'Finished', component: PluginEnrollSuccess, hide: true },
    ],
    FormMixin: EntraFormMixin,
  },
  {
    type: 'datadog',
    name: 'Datadog',
    description: 'Surface Teleport access requests as Datadog incidents.',
    icon: 'datadog',
    url: 'https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/datadog-hosted/',
    fullName: 'Datadog Incident Management',
    cloudHostable: true,
    selfHostable: true,
    tags: ['notifications'],
    Description: () => (
      <Text>
        <P>
          The Teleport integration with Datadog allows your team to treat
          Teleport permission requests as Datadog incidents and provides Datadog
          special actions to approve or deny permission requests.
        </P>
      </Text>
    ),
    Setup: () => (
      <Text>
        <P>
          You will need to provide your Datadog API key and an Application key
          that is associated with a Teleport service account. The Application
          key must provide <Mark>user_access_read</Mark> and{' '}
          <Mark>incident_write</Mark> access to allow the plugin to read and
          write Datadog incidents.
        </P>
        <ol>
          <li>
            Create a Datadog API key by following the{' '}
            <Link
              target="_blank"
              href="https://docs.datadoghq.com/account_management/api-app-keys/"
            >
              API Keys
            </Link>{' '}
            documentation. Copy and paste the key into the{' '}
            <Mark>Datadog API Key</Mark> field on this screen.
          </li>
          <li>
            Create a Datadog service account by following the{' '}
            <Link
              target="_blank"
              href="https://docs.datadoghq.com/account_management/org_settings/service_accounts/"
            >
              Service Accounts
            </Link>{' '}
            documentation. Name it <Mark>Teleport</Mark> and assign the{' '}
            <Mark>Datadog Standard Role</Mark>, or any Role that has the{' '}
            <Mark>Incidents Write</Mark>, <Mark>Teams Manage</Mark>, and{' '}
            <Mark>On-Call</Mark> permissions.
          </li>
          <li>
            After creating the service account, create a service account
            application key. You're welcome to limit the scope of the
            application key, but ensure that the scope includes{' '}
            <Mark>user_access_read</Mark>, <Mark>incident_write</Mark>,{' '}
            <Mark>teams_read</Mark>, and <Mark>on_call_read</Mark>. Copy and
            paste the key into <Mark>Datadog Application Key</Mark> field on
            this screen.
          </li>
        </ol>
      </Text>
    ),
    permissions: [
      {
        category: 'Access Management',
        permissions: [
          {
            title: 'user_access_read',
            description: 'View users and their roles and settings.',
          },
        ],
      },
      {
        category: 'Case and Incident Management',
        permissions: [
          {
            title: 'incident_write',
            description: 'Create, view, and manage incidents in Datadog.',
          },
        ],
      },
      {
        category: 'Teams',
        permissions: [
          {
            title: 'teams_read',
            description: 'Read Teams data.',
          },
        ],
      },
      {
        category: 'On-Call',
        permissions: [
          {
            title: 'on_call_read',
            description:
              'View On-Call teams, schedules, escalation policies and overrides.',
          },
        ],
      },
    ],
    FormMixin: () => {
      const [apiKey, setAPIKey] = useState('');
      const [applicationKey, setApplicationKey] = useState('');
      const [apiEndpoint, setAPIEndpoint] = useState<Option>();
      const [fallbackRecipient, setFallbackRecipient] = useState('');
      return (
        <>
          <FieldInput
            width="500px"
            label="Datadog API Key"
            name="apiKey" // must be the same name as expected by the backend as form value
            rule={requiredField('API Key Required')}
            value={apiKey}
            type="password"
            onChange={e => setAPIKey(e.target.value)}
            placeholder="abc-def...-123"
            toolTipContent="API Key is used to access the Datadog REST API"
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Datadog Application Key"
            name="applicationKey" // must be the same name as expected by the backend as form value
            rule={requiredField('Application Key Required')}
            value={applicationKey}
            type="password"
            onChange={e => setApplicationKey(e.target.value)}
            placeholder="abc-def...-123"
            toolTipContent="Application Key is used to access the Datadog REST API"
            mb={3}
          />
          <FieldSelect
            width="500px"
            label="API Endpoint"
            name="apiEndpoint" // must be the same name as expected by the backend as form value
            rule={requiredField<Option>('API Endpoint Required')}
            value={apiEndpoint}
            onChange={o => setAPIEndpoint(o as Option)}
            autoFocus
            options={[
              {
                value: 'https://api.datadoghq.com',
                label: 'US1 (https://api.datadoghq.com)',
              },
              {
                value: 'https://api.us3.datadoghq.com',
                label: 'US3 (https://api.us3.datadoghq.com)',
              },
              {
                value: 'https://api.us5.datadoghq.com',
                label: 'US5 (https://api.us5.datadoghq.com)',
              },
              {
                value: 'https://api.datadoghq.eu',
                label: 'EU1 (https://api.datadoghq.eu)',
              },
              {
                value: 'https://api.ap1.datadoghq.com',
                label: 'AP1 (https://api.ap1.datadoghq.com)',
              },
            ]}
            placeholder="Select API Endpoint"
            isSearchable
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Fallback Recipient"
            name="fallbackRecipient" // must be the same name as expected by the backend as form value
            rule={requiredField('Fallback Recipient Required')}
            value={fallbackRecipient}
            onChange={e => setFallbackRecipient(e.target.value)}
            placeholder="example@goteleport.com"
            toolTipContent="Fallback Recipient is the default recipient of Access Request notifications"
            mb={3}
          />
        </>
      );
    },
    NextSteps: () => {
      return (
        <Text>
          <P>
            For help with configuring access request notification routing rules,
            consult the{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/docs/admin-guides/access-controls/access-request-plugins/notification-routing-rules"
            >
              Notification Routing Rules
            </Link>{' '}
            section of Teleport's documentation.
          </P>
        </Text>
      );
    },
  },
  AwsIdentityCenterPlugin,
];

export const pluginMap = Object.fromEntries(plugins.map(p => [p.type, p]));

const StyledUl = styled.ul`
  margin: 0;
  padding-left: ${p => p.theme.space[4]}px;
`;
