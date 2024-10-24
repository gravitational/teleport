import React, { useState } from 'react';
import styled from 'styled-components';
import { Link as ReactRouterLink } from 'react-router-dom';

import { Text, Link, Flex, Box, H2 } from 'design';
import CardError from 'design/CardError';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';
import { CtaEvent } from 'teleport/services/userEvent';
import { ButtonLockedFeature } from 'teleport/components/ButtonLockedFeature';
import { OutlineWarn } from 'design/Alert/Alert';
import { Mark } from 'design/Mark';

import { UPGRADE_POLICY_URL } from 'teleport/services/sales';

import { P } from 'design/Text/Text';

import cfg from 'e-teleport/config';

import { SetUpScim } from './MultiStep/Okta/SetUpScim';
import { PluginEnrollSuccess } from './MultiStep/PluginEnrollSuccess';
import { CreateOkta } from './MultiStep/Okta/CreateOkta';
import { ImportUserGroupsAndApps } from './MultiStep/Okta/ImportUserGroupsAndApps/ImportUserGroupsAndApps';
import { FormDataField } from './MultiStep/Okta/types';

import { CreateEntra } from './MultiStep/Entra/CreateEntra';
import { FormMixin as EntraFormMixin } from './MultiStep/Entra/FormMixin';
import { RunScript } from './MultiStep/Entra/RunScript';

import { AwsIdentityCenterPlugin } from './MultiStep/AwsIdentityCenter/Plugin';

import type {
  SelfHostedPlugin,
  CloudHostablePlugin,
} from 'e-teleport/services/plugins';

export const plugins: (SelfHostedPlugin | CloudHostablePlugin)[] = [
  {
    type: 'slack',
    isOAuth: true,
    name: 'Slack',
    icon: 'slack',
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-slack/',
    fullName: 'Slack access request notifications',
    cloudHostable: true,
    selfHostable: false,
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
              'We will use email addresses to match Teleport users with their Slack profiles',
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
    icon: 'okta',
    url: 'https://goteleport.com/docs/application-access/okta/guide/',
    cloudHostable: true,
    selfHostable: true,
    fullName: 'Okta Integration',
    views: () => {
      if (
        cfg.oss.entitlements.OktaSCIM.enabled &&
        cfg.oss.entitlements.OktaUserSync.enabled
      ) {
        return [
          { title: 'Connect Okta', component: CreateOkta },
          { title: 'Import', component: ImportUserGroupsAndApps },
          { title: 'Set Up SCIM', component: SetUpScim },
          { title: 'Finished', component: PluginEnrollSuccess, hide: true },
        ];
      }
      return [];
    },
    Description: () => {
      return (
        <Box mb={3}>
          <Text>
            The Okta integration can sync users, applications, and groups with
            your Teleport instance and set up an SSO integration.
          </Text>
          <H2 mt={3}>Included with Teleport:</H2>
          <StyledUl>
            <li>
              <strong>App Synchronization</strong>: Routinely synchronizes Okta
              apps and groups with Teleport
            </li>
            <li>
              <strong>User Access</strong>: Grants access to Okta apps and
              groups based on Teleport permissions
            </li>
            <li>
              <strong>Access Requests</strong>: Enables users to request
              temporary access to Okta apps and groups
            </li>
            <li>
              <strong>SSO integration</strong>: A SAML SSO connector that grants
              Okta users the default role of <em>requester</em>
            </li>
          </StyledUl>
          <H2 mt={3}>Included with Teleport Identity:</H2>
          <StyledUl>
            <li>
              <strong>User Synchronization</strong>: Routinely synchronizes
              users with Teleport so the Teleport user list always includes your
              full Okta user list
            </li>
            <li>
              <strong>User Group Synchronization</strong>: Syncs Okta user
              groups with Teleport Access Lists for simple permissions
              management and auditing capabilities
            </li>
          </StyledUl>
          {!cfg.oss.entitlements.OktaUserSync.enabled && (
            <ButtonLockedFeature
              event={CtaEvent.CTA_OKTA_USER_SYNC}
              width={'460px'}
              mt={2}
              mb={3}
            >
              Unlock User Synchronization with Teleport Identity
            </ButtonLockedFeature>
          )}
        </Box>
      );
    },
    Setup: () => (
      <Box>
        <Text mt={1}>
          Generate an API key so you can set up the Okta plugin:
        </Text>
        <ol css={{ margin: 0 }}>
          <li>
            Create an admin role for the Teleport Okta Service by following the{' '}
            <Link
              target="_blank"
              href="https://help.okta.com/en-us/content/topics/security/custom-admin-role/create-role.htm"
            >
              Okta documentation
            </Link>
            . The role must have the following permissions:
            <ul>
              <li>
                User permissions:
                <ul>
                  <li>View users and their details</li>
                  <li>Edit users' group membership</li>
                  <li>Edit users' application assignments</li>
                </ul>
              </li>
              <li>
                Group permissions:
                <ul>
                  <li>View groups and their details</li>
                  <li>Manage group membership</li>
                </ul>
              </li>
              <li>
                Application permissions:
                <ul>
                  <li>View applications and their details</li>
                  <li>Edit application's user assignments</li>
                </ul>
              </li>
            </ul>
          </li>
          <li>
            Follow the{' '}
            <Link
              target="_blank"
              href="https://help.okta.com/en-us/content/topics/security/custom-admin-role/create-admin-role-assignment-by-admin.htm"
            >
              Okta documentation
            </Link>{' '}
            to create a user for the Teleport Okta Service and assign two roles
            to the user:
            <ul>
              <li>
                The built-in Group Membership Admin role, which can create API
                tokens
              </li>
              <li>The role you created earlier</li>
            </ul>
          </li>
          <li>
            Sign in to Okta as the user you created and follow the{' '}
            <Link
              target="_blank"
              href="https://help.okta.com/en-us/content/topics/security/api.htm"
            >
              Okta documentation
            </Link>{' '}
            to generate an API token, which inherits the permissions of the
            user. Paste the API token into the form at the bottom of this
            screen.
          </li>
        </ol>
      </Box>
    ),
    FormMixin: () => {
      const [url, setUrl] = useState('');
      const [token, setToken] = useState('');

      return (
        <>
          <FieldInput
            width="500px"
            label="Okta Domain"
            name={FormDataField.OrgUrl}
            rule={requiredField('Okta domain Required')}
            value={url}
            onChange={e => setUrl(e.target.value)}
            placeholder="examplecompanyname.okta.com"
            toolTipContent="Okta domain is used for API communication"
            mb={3}
          />
          <FieldInput
            mb={5}
            width="500px"
            label="API Token"
            name={FormDataField.ApiToken}
            type="password"
            rule={requiredField('API Token Required')}
            value={token}
            onChange={e => setToken(e.target.value)}
            placeholder="00QCjAl4MlV-WPXM...0HmjFx-vbGua"
            toolTipContent="Okta API tokens are used to authenticate requests to Okta APIs"
          />
          <OutlineWarn
            css={{ justifyContent: 'normal', maxWidth: '620px' }}
            linkColor="buttons.link.default"
          >
            <P>
              Enabling Okta integration will make Teleport take ownership over
              app and group assignments in Okta and can make changes within Okta
              based on Teleport's RBAC configuration.
            </P>
            <P>
              Specifically, access to Okta apps is governed by Teleport roles{' '}
              <Link
                target="_blank"
                href="https://goteleport.com/docs/application-access/controls/#configuring-application-labels-in-roles"
              >
                app_labels
              </Link>
              {'. '}
              Ensure that your users do not have roles with wildcard{' '}
              <Link
                target="_blank"
                href="https://goteleport.com/docs/application-access/controls/#configuring-application-labels-in-roles"
              >
                app_labels
              </Link>
              , which otherwise will result into those users being assigned to
              all Okta applications.
            </P>
            <P>
              To limit the scope of this integration, you can constrain Okta
              access token to a subset of apps and groups by using{' '}
              <Link
                target="_blank"
                href="https://help.okta.com/en-us/content/topics/security/custom-admin-role/create-resource-set.htm"
              >
                Okta resource set
              </Link>
              .
            </P>
          </OutlineWarn>
        </>
      );
    },
    NextSteps: () => {
      return (
        <>
          <P>
            After enabling the Okta integration, create an import rule to
            configure the applications that Teleport imports from Okta. See the{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/docs/application-access/okta/reference/"
            >
              Teleport documentation
            </Link>{' '}
            for details.
          </P>

          <P>
            It may take a while before all applications and groups are synced to
            Teleport.
          </P>
        </>
      );
    },
  },
  {
    type: 'opsgenie',
    name: 'Opsgenie',
    icon: 'opsgenie',
    url: 'https://goteleport.com/docs/access-controls/access-requests/resource-requests/', // TODO(lisa): change to opsgenie docs (wip)
    cloudHostable: true,
    selfHostable: true,
    fullName: 'Opsgenie access request notifications',
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
    url: 'https://goteleport.com/docs/access-controls/device-trust/jamf-integration/?scope=enterprise',
    cloudHostable: true,
    selfHostable: false,
    disabledIfNoMdmSupport: true,
    fullName: 'Jamf Integration for Device Trust',
    Description: () => (
      <Text>
        <P>
          Jamf integration updates trusted devices in Teleport to match
          available devices in your Jamf inventory. For more details, see our
          docs page about{' '}
          <Link
            href="https://goteleport.com/docs/access-controls/device-trust/jamf-integration/?scope=enterprise"
            target="_blank"
          >
            Device Trust and the Jamf Integration.
          </Link>
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
    type: 'servicenow',
    name: 'ServiceNow',
    icon: 'servicenow',
    url: 'https://goteleport.com/docs/access-controls/access-requests/resource-requests/',
    cloudHostable: true,
    selfHostable: true,
    fullName: 'ServiceNow Integration',
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
              href="https://docs.servicenow.com/en-US/bundle/vancouver-it-service-management/page/product/incident-management/task/req-itsm-roles-inci-mgmt.html"
            >
              ITSM Roles plugin
            </Link>
            , which enables the “sn_incident_read” and “sn_incident_write”
            roles.
          </li>
          <li>
            <Link
              target="_blank"
              href="https://docs.servicenow.com/bundle/vancouver-platform-administration/page/administer/roles/task/t_CreateARole.html"
            >
              Create a ServiceNow role
            </Link>{' '}
            that we will later assign to the user account for the Teleport
            integration.
          </li>
          <li>
            <Link
              target="_blank"
              href="https://docs.servicenow.com/bundle/vancouver-platform-administration/page/administer/roles/task/t_AddARoleToAnExistingRole.html"
            >
              Edit the role you created
            </Link>{' '}
            to add the “sn_incident_read” and “sn_incident_write” roles.
          </li>
          <li>
            Follow the{' '}
            <Link
              target="_blank"
              href="https://docs.servicenow.com/en-US/bundle/vancouver-platform-administration/page/administer/users-and-groups/task/t_CreateAUser.html"
            >
              ServiceNow documentation
            </Link>{' '}
            to create a ServiceNow user. Paste the name and password of the user
            in the form at the bottom of this page.
          </li>
          <li>
            <Link
              target="_blank"
              href="https://docs.servicenow.com/bundle/vancouver-platform-administration/page/administer/users-and-groups/task/t_AssignARoleToAUser.html"
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
    fullName: 'Jira access request management',
    icon: 'jira',
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-jira',
    cloudHostable: true,
    selfHostable: true,
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
              href="https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-jira/#step-36-set-up-your-jira-project"
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
    icon: 'pagerduty',
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-pagerduty/',
    fullName: 'PagerDuty access request management',
    cloudHostable: true,
    selfHostable: true,
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
              href="https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-pagerduty/#step-28-define-rbac-resources"
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
    icon: 'email',
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-email/',
    cloudHostable: false,
    selfHostable: true,
  },
  {
    type: 'discord',
    name: 'Discord',
    fullName: 'Discord access request notifications',
    icon: 'discord',
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-discord/',
    cloudHostable: true,
    selfHostable: false,
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
            href="https://discord.com/developers/docs/getting-started"
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
    icon: 'mattermost',
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-mattermost/',
    fullName: 'Mattermost access request notifications',
    cloudHostable: true,
    selfHostable: true,
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
              href="https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-mattermost/?scope=enterprise#step-18-define-rbac-resources"
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
    icon: 'microsoftteams',
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-msteams/',
    cloudHostable: false,
    selfHostable: true,
  },
  {
    type: 'entra-id',
    name: 'Microsoft Entra ID',
    icon: 'entraid',
    url: '',
    fullName: 'Entra ID directory synchronization',
    cloudHostable: true,
    selfHostable: true,
    // todo (michellescripts) replace with new entitlement `EntraIDSync`
    requiresIgs: true,

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
        <H2 mt={3}>Included with Teleport Identity:</H2>
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
        <H2 mt={3}>Included with Teleport Policy:</H2>
        <StyledUl>
          <li>
            <strong>Access Graph integration</strong>: analyze your Entra ID
            directory and SSO applications using Teleport Access Graph.
          </li>
        </StyledUl>
        {(!cfg.oss.entitlements.Policy.enabled || !cfg.oss.isPolicyEnabled) && (
          <ButtonLockedFeature
            event={CtaEvent.CTA_ENTRA_ID}
            width={'460px'}
            mt={2}
            mb={3}
            url={UPGRADE_POLICY_URL}
          >
            Unlock Access Graph integration with Teleport Policy
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
    icon: 'datadog',
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/datadog-hosted/',
    fullName: 'Datadog Incident Management',
    cloudHostable: true,
    selfHostable: true,
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
            <Mark>Incidents Write</Mark> permission.
          </li>
          <li>
            After creating the service account, create a service account
            application key. You're welcome to limit the scope of the
            application key, but ensure that the scope includes{' '}
            <Mark>user_access_read</Mark> and <Mark>incident_write</Mark>. Copy
            and paste the key into <Mark>Datadog Application Key</Mark> field on
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
