import React, { useState } from 'react';
import styled from 'styled-components';
import { Link as ReactRouterLink } from 'react-router-dom';

import { Text, Link, Flex, Box } from 'design';
import CardError from 'design/CardError';
import * as Icons from 'design/Icon';
import oktaIcon from 'design/assets/images/icons/okta.svg';
import opsgenieIcon from 'design/assets/images/icons/opsgenie.svg';
import serviceNowIcon from 'design/assets/images/icons/servicenow.svg';
import slackIcon from 'design/assets/images/icons/slack.svg';
import pagerdutyIcon from 'design/assets/images/icons/pagerduty.svg';
import emailIcon from 'design/assets/images/icons/email.svg';
import jiraIcon from 'design/assets/images/icons/jira.svg';
import discordIcon from 'design/assets/images/icons/discord.svg';
import mattermostIcon from 'design/assets/images/icons/mattermost.svg';
import msteamsIcon from 'design/assets/images/icons/msteams.svg';
import JamfIcon from 'design/assets/images/icons/jamf.svg';
import FieldInput from 'shared/components/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';
import { IntegrationEnrollKind } from 'teleport/services/userEvent';
import { PluginKind } from 'teleport/services/integrations';

import cfg from 'e-teleport/config';

type Permission = {
  title: string;
  description?: string;
};

type CategoryPermissions = {
  category: string;
  permissions: Permission[];
};

// EnrollSuccessResponse contains the necessary data to guide the user
// after the plugin is connected (in `NextSteps` component).
// This is equivalent to `pluginOnboardingCookieNonSensitiveData` in e/lib/web/plugins.go
export type EnrollSuccessResponse = {
  slack?: {
    fallback_channel: string;
  };
};

export type PluginBase = {
  type: PluginKind;
  name: string;
  icon: string;
  url: string;

  // isOAuth describes a plugin that are authenticated
  // via OAuth.
  isOAuth?: boolean;

  /**
   * Describes whether the plugin can be hosted by Teleport Cloud.
   */
  cloudHostable: boolean;

  /**
   * Describes whether the plugin can be self hosted.
   */
  selfHostable: boolean;

  disableForTeam?: boolean;
};

/**
 * SelfHostedPlugin describes a plugin that can be self hosted.
 */
export type SelfHostedPlugin = PluginBase & {
  cloudHostable: false;
  selfHostable: true;
  disableForTeam?: true;
};

/**
 * CloudHostablePlugin describes a plugin that can be hosted by Teleport Cloud.
 * CloudHostablePlugins might also be self hostable, determined by the `selfHostable`
 * field.
 */
export type CloudHostablePlugin = PluginBase & {
  cloudHostable: true;
  /** selfHostable is true for cloud plugins that can be self hosted as well. */
  selfHostable: boolean;

  // For each hosted plugin, these describe additional elements in the enroll page.
  fullName: string;
  Description?: () => JSX.Element;
  Setup?: () => JSX.Element;
  FormMixin?: () => JSX.Element;
  NextSteps?: (props: { successData?: EnrollSuccessResponse }) => JSX.Element;
  permissions?: CategoryPermissions[];
  disableForTeam?: boolean;
};

export const plugins: (SelfHostedPlugin | CloudHostablePlugin)[] = [
  {
    type: 'slack',
    isOAuth: true,
    name: 'Slack',
    icon: slackIcon,
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/slack',
    fullName: 'Slack access request notifications',
    cloudHostable: true,
    selfHostable: false,
    Description: () => (
      <Text>
        <p>
          The Slack integration receives access requests from Teleport and posts
          them as Slack messages to alert reviewers.
        </p>
        <p>
          If an access request includes suggested reviewers, the Slack
          integration will add these to the list of channels to notify. If a
          suggested reviewer is an email address, the Slack integration will
          look up the the direct message channel for that address and post a
          message in that channel. Otherwise, the integration will post messages
          in the default channel that you select on this screen.
        </p>
        <p>
          Please note that if you do not have permissions to add new apps to
          your Slack workspace, you will need to request approval in the next
          step. Once that approval is given, Slackbot will notify you within
          your workspace, and you will be able to connect Slack and Teleport by
          coming back to this view and clicking the “Connect Slack” button
          again.
        </p>
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
        <InputIconContainer style={{ position: 'relative' }}>
          <StyledFieldInput
            width="260px"
            label="Default Channel"
            name="fallback_channel" // must be the same name as expected by the backend as form value
            rule={requiredField('Default channel must be specified')}
            value={channel}
            onChange={e => setChannel(e.target.value)}
            autoFocus
            placeholder="access-requests"
            toolTipContent="The default channel will receive all notifications about access requests. Request notifications will also be sent directly to assigned reviewers (if any)."
          />
          <StyledHashtagIcon color="text.primary" size="medium" />
        </InputIconContainer>
      );
    },
    NextSteps: ({ successData }) => {
      const fallbackChannel = successData?.slack?.fallback_channel;
      if (!fallbackChannel) {
        return <CardError>Failed to parse the response.</CardError>;
      }
      return (
        <Text typography="body1">
          As the final step, you should invite the "Teleport Cloud" application
          to channel <strong>{fallbackChannel}</strong> in your Slack workspace.
        </Text>
      );
    },
  },
  {
    type: 'okta',
    name: 'Okta',
    icon: oktaIcon, // TODO(lisa): update all these icons to SVGIcon for theme friendly
    url: 'https://goteleport.com/docs/application-access/okta/guide/',
    cloudHostable: true,
    selfHostable: false,
    fullName: 'Okta Integration',
    Description: () => (
      <Text>
        <p>
          The Teleport Okta integration synchronizes Okta and Teleport users,
          apps and permissions.
        </p>
        <ul>
          <li>
            <strong>SSO integration</strong>: Installing the Okta integration
            creates an Okta applicaton with a SAML SSO connector called{' '}
            <em>okta-integration</em> that grants your Okta users access to the
            Teleport cluster with the
            <em>requester</em> role.
          </li>
          <li>
            <strong>App Synchronization</strong>: Routinely synchronizes Okta
            applications and groups with Teleport.
          </li>
          <li>
            <strong>User Synchronization</strong>: Routinely synchronizes Okta
            users with Teleport. The Okta user profile data is exposed to
            Teleport via user traits.
          </li>
          <li>
            <strong>User Access</strong>: Longer lived permissions that grant
            users access to the Okta applications and groups based on their
            Teleport access permissions.
          </li>
          <li>
            <strong>Access Requests</strong>: Short lived permissions to request
            temporary access to Okta applications and user groups based on their
            Teleport access permissions.
          </li>
        </ul>
      </Text>
    ),
    permissions: [
      {
        category: 'Applications',
        permissions: [
          {
            title: 'Manage Applications',
            description: 'Installing SAML SSO connector',
          },
          {
            title: "Edit application's user assignments",
            description: 'Synchronizing application access',
          },
        ],
      },
      {
        category: 'Users',
        permissions: [
          {
            title: 'View Users and their details',
            description: 'Synchronizing Teleport users with Okta users',
          },
        ],
      },
      {
        category: 'Groups',
        permissions: [
          {
            title: 'View Groups',
            description: 'Synchronizing application access',
          },
          {
            title: "Edit groups' application assignments",
            description: 'Synchronizing application access',
          },
        ],
      },
    ],
    FormMixin: () => {
      const [url, setUrl] = useState('');
      const [token, setToken] = useState('');
      return (
        <>
          <FieldInput
            width="500px"
            label="Organization URL"
            name="orgURL" // must be the same name as expected by the backend as form value
            rule={requiredField('Organization URL Required')}
            value={url}
            onChange={e => setUrl(e.target.value)}
            autoFocus
            placeholder="examplecompanyname.okta.com"
            toolTipContent="Okta organization URL are used for API communication"
            mb={3}
          />
          <FieldInput
            width="500px"
            label="API Token"
            name="apiToken" // must be the same name as expected by the backend as form value
            type="password"
            rule={requiredField('API Token Required')}
            value={token}
            onChange={e => setToken(e.target.value)}
            placeholder="00QCjAl4MlV-WPXM...0HmjFx-vbGua"
            toolTipContent="Okta API tokens are used to authenticate requests to Okta APIs"
          />
        </>
      );
    },
    NextSteps: () => {
      return (
        <Text typography="body1">
          <p>
            After enabling the Okta integration, create an import rule to
            configure the applications that Teleport imports from Okta. See the{' '}
            <a href="https://goteleport.com/docs/application-access/okta/reference/">
              Teleport documentation
            </a>{' '}
            for details.
          </p>

          <p>
            It may take a while before all applications and groups are synced to
            Teleport.
          </p>
        </Text>
      );
    },
  },
  {
    type: 'opsgenie',
    name: 'Opsgenie',
    icon: opsgenieIcon, // TODO(lisa): update all these icons to SVGIcon for theme friendly
    url: 'https://goteleport.com/docs/access-controls/access-requests/resource-requests/', // TODO(lisa): change to opsgenie docs (wip)
    cloudHostable: true,
    selfHostable: true,
    fullName: 'Opsgenie access request notifications',
    Description: () => (
      <Text>
        <p>
          Integrating with Opsgenie allows Teleport access requests to show up
          as alerts in the specified Opsgenie schedule.
        </p>
        <p>
          You will need to provide an API key with the following permissions
          “Read Access” and “Create and Update Access”.
        </p>
      </Text>
    ),
    Setup: () => (
      <Text>
        <p>
          Generate an API key that the Opsgenie plugin will use to create and
          modify alerts as well as list users, services, and on-call policies.
        </p>

        <ol>
          <li>
            Follow the instructions in the{' '}
            <a href="https://support.atlassian.com/opsgenie/docs/api-key-management/">
              Opsgenie documentation
            </a>
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
            rule={requiredField('API Endpoint Required')}
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
            toolTipContent="API Key is used to request to Opsgenie REST API and requires the permissons: 'Read Access' and 'Create and Update Access'"
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
    icon: JamfIcon,
    url: 'https://goteleport.com/docs/access-controls/device-trust/jamf-integration/?scope=enterprise',
    cloudHostable: true,
    selfHostable: false,
    disableForTeam: true,
    fullName: 'Jamf Integration for Device Trust',
    Description: () => (
      <Text>
        <p>
          Jamf integration updates trusted devices in Teleport to match
          available devices in your Jamf inventory. For more details, see our
          docs page about{' '}
          <Link
            href="https://goteleport.com/docs/access-controls/device-trust/jamf-integration/?scope=enterprise"
            target="_blank"
          >
            Device Trust and the Jamf Integration.
          </Link>
        </p>
      </Text>
    ),
    Setup: () => (
      <Text>
        <p>Create a read-only Jamf user for inventory sync:</p>

        <ol>
          <li>
            Access {'https://yourtenant.jamfcloud.com/accounts.html'}, replacing
            “yourtenant” with your Jamf Pro account URL.{' '}
          </li>

          <li>
            {' '}
            Create a new Standard Account with the following settings:
            <ul>
              <li> Username: teleport (change as desired)</li>
              <li> Access Level: Full Access</li>
              <li> Privilege Set: Custom</li>
              <li> Access Status: Enabled</li>
              <li> Password: (a strong password of your choice)</li>
              <li> Privileges:</li>
              <ul>
                <li>Advanced Computer Searches: Read</li>
                <li>Computers: Read</li>
              </ul>
            </ul>
          </li>
          <li>
            Take note of the user and password you created in order to configure
            the Jamf integration on this screen.
          </li>
        </ol>
      </Text>
    ),
    permissions: [
      {
        category: 'API Access',
        permissions: [
          {
            title: 'Read-only access to Jamf API.',
            description:
              'Teleport will authenticate to Jamf API using Jamf account credential (username + password).',
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
      const [username, setUsername] = useState('');
      const [password, setPassword] = useState('');
      const [apiEndpoint, setApiEndpoint] = useState('');
      return (
        <InputIconContainer style={{ position: 'relative' }}>
          <StyledFieldInput
            width="500px"
            label="Jamf API Endpoint"
            name="apiEndpoint" // must be the same name as expected by the backend as form value
            rule={requiredField('API endpoint must be specified')}
            value={apiEndpoint}
            onChange={e => setApiEndpoint(e.target.value)}
            autoFocus
            placeholder="yourserver.jamfcloud.com"
            toolTipContent="URL of Jamf API. (e.g. https://yourtenant.jamfcloud.com)"
          />
          <StyledFieldInput
            width="500px"
            label="Jamf Account Username"
            name="username" // must be the same name as expected by the backend as form value
            rule={requiredField('Username must be specified')}
            value={username}
            onChange={e => setUsername(e.target.value)}
            placeholder="username"
            toolTipContent="Username of the account that will be used to authenticate with Jamf API. We recommend using a read-only account."
          />
          <StyledFieldInput
            width="500px"
            label="Jamf Account Password"
            name="password" // must be the same name as expected by the backend as form value
            rule={requiredField('Password must be specified')}
            value={password}
            onChange={e => setPassword(e.target.value)}
            placeholder="password"
            type="password"
            toolTipContent="Password of the account that will be used to authenticate with Jamf API.  We recommend using a read-only account."
          />
        </InputIconContainer>
      );
    },
    NextSteps: () => {
      return (
        <Text typography="body1">
          Jamf integration is configured for your cluster. Depending on the size
          of your Jamf inventory, it may take a few minutes to sync with{' '}
          <ReactRouterLink to={cfg.routes.deviceTrust}>
            Trusted Devices
          </ReactRouterLink>{' '}
          in Teleport.
        </Text>
      );
    },
  },
  {
    type: 'servicenow',
    name: 'ServiceNow',
    icon: serviceNowIcon,
    url: 'https://goteleport.com/docs/access-controls/access-requests/resource-requests/',
    cloudHostable: true,
    selfHostable: true,
    fullName: 'ServiceNow Integration',
    Description: () => (
      <Text>
        <p>
          ServiceNow plugin creates ServiceNow incidents for Teleport access
          requests.
        </p>
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
    FormMixin: () => {
      const [username, setUsername] = useState('');
      const [password, setPassword] = useState('');
      const [apiEndpoint, setApiEndpoint] = useState('');
      const [closeCode, setCloseCode] = useState('');
      return (
        <InputIconContainer style={{ position: 'relative' }}>
          <StyledFieldInput
            width="500px"
            label="ServiceNow API Endpoint"
            name="apiEndpoint" // must be the same name as expected by the backend as form value
            rule={requiredField('API endpoint must be specified')}
            value={apiEndpoint}
            onChange={e => setApiEndpoint(e.target.value)}
            autoFocus
            placeholder="yourserver.servicenowcloud.com"
            toolTipContent="URL of ServiceNow API. (e.g. https://example-servicenow-instance.com)"
          />
          <StyledFieldInput
            width="500px"
            label="ServiceNow Account Username"
            name="username" // must be the same name as expected by the backend as form value
            rule={requiredField('Username must be specified')}
            value={username}
            onChange={e => setUsername(e.target.value)}
            placeholder="Username"
            toolTipContent="Username of the account that will be used to authenticate with Servicenow API."
          />
          <StyledFieldInput
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
          <StyledFieldInput
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
        </InputIconContainer>
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
    icon: jiraIcon,
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
        <p>
          The Teleport Jira integration allows you to manage Teleport access
          requests using Jira tickets.
        </p>
      </Text>
    ),
    Setup: () => (
      <Text>
        <ol>
          <li>
            Follow the{' '}
            <a href="https://support.atlassian.com/jira-software-cloud/docs/create-a-new-project/">
              Jira documentation
            </a>{' '}
            to create a project. Ensure that the project has the following
            attributes:
            <ul>
              <li>
                Uses the{' '}
                <a href="https://www.atlassian.com/software/jira/templates/kanban">
                  Kanban
                </a>{' '}
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
            The Teleport Jira integration expects tasks your project board to
            include a field called “teleportAccessRequestId”, which it uses to
            track individual access requests. This prevents users from tampering
            with or forging access requests. Follow the{' '}
            <a href="https://support.atlassian.com/jira-cloud-administration/docs/create-a-custom-field/">
              Jira documentation
            </a>{' '}
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
            <a href="https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/#Create-an-API-token">
              Jira documentation
            </a>
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
          <p>
            Teleport will create issues in your Jira project in response to
            access requests.
          </p>
          <p>
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
          </p>
        </Text>
      );
    },
  },
  {
    type: 'pagerduty',
    name: 'PagerDuty',
    icon: pagerdutyIcon,
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-pagerduty/',
    fullName: 'PagerDuty access request management',
    cloudHostable: true,
    selfHostable: false,
    Description: () => (
      <Text>
        <p>
          The Teleport integration with PagerDuty allows your team to treat
          Teleport permission requests as Pagerduty incidents and provides
          Pagerduty saecial actions to approve or deny permission requests.
        </p>
      </Text>
    ),
    Setup: () => (
      <Text>
        <p>
          You will need to generate an API key for the PagerDuty integration to
          use to create and modify incidents as well as list users, services,
          and on-call policies.
        </p>

        <ol>
          <li>
            Follow the{' '}
            <a href="https://support.pagerduty.com/docs/api-access-keys#generate-a-general-access-rest-api-key">
              PagerDuty documentation
            </a>{' '}
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
          <p>
            Be sure to configure the `pagerduty_notify_service` and
            `pagerduty_services` annotations in the Teleport roles you want
            PagerDuty to manage.
          </p>
          <p>
            For more infomation, consult the <em>Define RBAC Resources</em>{' '}
            section of the Teleport{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-pagerduty/#step-28-define-rbac-resources"
            >
              Access Requests with PagerDuty
            </Link>{' '}
            guide.
          </p>
        </Text>
      );
    },
  },
  {
    type: 'email',
    name: 'Email',
    icon: emailIcon,
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/email',
    cloudHostable: false,
    selfHostable: true,
  },
  {
    type: 'discord',
    name: 'Discord',
    fullName: 'Discord access request notifications',
    icon: discordIcon,
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-discord',
    cloudHostable: true,
    selfHostable: false,
    Description: () => (
      <Text>
        <p>
          The Discord integration receives access request events from the
          Teleport Auth Service, formats them into Discord messages, and sends
          them to the Discord API to post them in your guild (Discord server).
        </p>
      </Text>
    ),
    Setup: () => (
      <ol>
        <li>
          Follow the{' '}
          <a href="https://discord.com/developers/docs/getting-started">
            Discord documentation
          </a>{' '}
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
            rule={requiredField('Channel')}
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
    icon: mattermostIcon,
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-mattermost/',
    fullName: 'Mattermost access request notifications',
    cloudHostable: true,
    selfHostable: false,
    Description: () => (
      <Text>
        <p>
          The Mattermost integration receives access requests from Teleport and
          posts them as Mattermost messages to alert reviewers.
        </p>

        <p>
          If an access request includes suggested reviewers, the Mattermost
          integration will add these to the list of channels to notify. If a
          suggested reviewer is an email address, the integration will look up
          the the direct message channel for that address and post a message in
          that channel. Otherwise, the integration will post the message to the
          default channel that you specify on this screen.
        </p>
      </Text>
    ),
    Setup: () => (
      <Text>
        <ol>
          <li>
            Follow the Mattermost{' '}
            <a href="https://developers.mattermost.com/integrate/reference/bot-accounts/#user-interface-ui">
              documentation
            </a>{' '}
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
          <p>
            For help with configuring roles for access requests, consult the{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-mattermost/?scope=enterprise#step-18-define-rbac-resources"
            >
              Define RBAC Resources
            </Link>{' '}
            section of Teleport's Mattermost guide.
          </p>
        </Text>
      );
    },
  },
  {
    type: 'msteams',
    name: 'Microsoft Teams',
    icon: msteamsIcon,
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/msteams',
    cloudHostable: false,
    selfHostable: true,
  },
];

export const pluginMap = Object.fromEntries(plugins.map(p => [p.type, p]));

const InputIconContainer = styled.div`
  position: relative;
`;

const StyledHashtagIcon = styled(Icons.Hashtag)`
  display: inline;
  padding: 0 6px;
  position: absolute;
  top: 40px;
`;

const StyledFieldInput = styled(FieldInput)`
  input {
    padding-left: 23px; /* Make room for an icon */
  }
`;

export function pluginTypeToIntegrationEnrollKind(p: PluginKind) {
  switch (p) {
    case 'discord':
      return IntegrationEnrollKind.Discord;
    case 'email':
      return IntegrationEnrollKind.Email;
    case 'jira':
      return IntegrationEnrollKind.Jira;
    case 'mattermost':
      return IntegrationEnrollKind.Mattermost;
    case 'msteams':
      return IntegrationEnrollKind.MsTeams;
    case 'pagerduty':
      return IntegrationEnrollKind.PagerDuty;
    case 'slack':
      return IntegrationEnrollKind.Slack;
    case 'okta':
      return IntegrationEnrollKind.Okta;
    case 'servicenow':
      return IntegrationEnrollKind.ServiceNow;
    case 'jamf':
      return IntegrationEnrollKind.Jamf;
    case 'opsgenie':
      return IntegrationEnrollKind.OpsGenie;
    default:
      return IntegrationEnrollKind.Unspecified;
  }
}
