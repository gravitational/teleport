import React, { useState } from 'react';
import styled from 'styled-components';
import { Link as ReactRouterLink } from 'react-router-dom';

import { Text, Link } from 'design';
import CardError from 'design/CardError';
import * as Icons from 'design/Icon';
import oktaIcon from 'design/assets/images/icons/okta.svg';
import opsgenieIcon from 'design/assets/images/icons/opsgenie.svg';
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
};

export type SelfHostedPlugin = PluginBase & {
  hosted: false;
};

export type HostedPlugin = PluginBase & {
  hosted: true;

  // For each hosted plugin, these describe additional elements in the enroll page.
  fullName: string;
  Description?: () => JSX.Element;
  FormMixin?: () => JSX.Element;
  NextSteps?: (props: { successData?: EnrollSuccessResponse }) => JSX.Element;
  permissions?: CategoryPermissions[];
};

export const plugins: (SelfHostedPlugin | HostedPlugin)[] = [
  {
    type: 'slack',
    isOAuth: true,
    name: 'Slack',
    icon: slackIcon,
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/slack',
    hosted: true,
    fullName: 'Slack Access Notifications',
    Description: () => (
      <Text>
        <p>
          Your Slack integration will match Slack and Teleport emails for
          Teleport reviewers and alert them whenever a teammate makes an access
          request. To do that, the integration will need you* to approve the
          following permissions:
        </p>
        <p>
          *Please note that if you do not have permissions to add new apps to
          your Slack workspace, you will need to request approval in the next
          step. Once that approval is given, Slackbot will notify you within
          your workspace, and you will be able to connect Slack and Teleport by
          coming back to this view and clicking the “Connect Slack” button
          again.
        </p>
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
            label="Default channel"
            name="fallback_channel" // must be the same name as expected by the backend as form value
            rule={requiredField('Default channel must be specified')}
            value={channel}
            onChange={e => setChannel(e.target.value)}
            autoFocus
            placeholder="access-requests"
            toolTipContent="The default channel will receive all notifications about access requests. Request notifications will also be sent directly to assigned reviewers (if any)."
          />
          <StyledHashtagIcon color="text.primaryInverse" />
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
    hosted: true,
    fullName: 'Okta Integration',
    Description: () => (
      <Text>
        Teleport provides an Okta Service that is responsible for dealing with
        all interactions with Okta.
      </Text>
    ),
    permissions: [
      {
        category: 'Synchronization',
        permissions: [
          {
            title:
              'Runs every 2 minutes, Okta Service will import both Okta applications and user groups into Teleport',
          },
        ],
      },
      {
        category: 'User Access',
        permissions: [
          {
            title: 'Longer lived permissions',
          },
          {
            title:
              'Grant access to Okta applications and user groups that users have access to within Teleport',
          },
        ],
      },
      {
        category: 'Access Requests',
        permissions: [
          {
            title: 'Short lived permissions',
          },
          {
            title:
              'Request temporary access to Okta applications and user groups',
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
          It may take a while before all applications and groups are synced to
          Teleport.
        </Text>
      );
    },
  },
  {
    type: 'opsgenie',
    name: 'Opsgenie',
    icon: opsgenieIcon, // TODO(lisa): update all these icons to SVGIcon for theme friendly
    url: 'https://goteleport.com/docs/access-controls/access-requests/resource-requests/', // TODO(lisa): change to opsgenie docs (wip)
    hosted: true,
    fullName: 'Opsgenie Alerts',
    Description: () => (
      <Text>
        <p>
          Integrating with Opsgenie allows Teleport access requests to show up
          as alerts in the specified Opsgenie schedule.
        </p>
        <p>
          You will need to provide an API key with the following permissions
          'Read Access' and 'Create and Update Access'
        </p>
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
    hosted: true,
    fullName: 'Jamf Integration for Device Trust',
    Description: () => (
      <Text>
        <p>
          Jamf plugin updates trusted devices in Teleport to match available
          devices in your Jamf inventory. For more details, see our docs page
          about{' '}
          <Link
            href="https://goteleport.com/docs/access-controls/device-trust/jamf-integration/?scope=enterprise"
            target="_blank"
          >
            Device Trust and the Jamf Integration.
          </Link>
        </p>
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
          Jamf plugin is configured for your cluster. Depending on the size of
          your Jamf inventory, it may take a few minutes to sync with{' '}
          <ReactRouterLink to={cfg.routes.deviceTrust}>
            Trusted Devices
          </ReactRouterLink>{' '}
          in Teleport.
        </Text>
      );
    },
  },
  {
    type: 'pagerduty',
    name: 'PagerDuty',
    icon: pagerdutyIcon,
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-pagerduty/',
    hosted: true,
    fullName: 'PagerDuty Alerts',
    Description: () => (
      <Text>
        <p>
          A Teleport integration with PagerDuty allows your team to treat
          Teleport permission requests as Pagerduty incidents, and provides
          Pagerduty special actions to approve or deny permission requests.
        </p>
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
    hosted: false,
  },
  {
    type: 'jira',
    name: 'Jira',
    icon: jiraIcon,
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/jira',
    hosted: false,
  },
  {
    type: 'discord',
    name: 'Discord',
    icon: discordIcon,
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/discord',
    hosted: false,
  },
  {
    type: 'mattermost',
    name: 'Mattermost',
    icon: mattermostIcon,
    url: 'https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-mattermost/',
    hosted: true,
    fullName: 'Mattermost Access Notifications',
    Description: () => (
      <Text>
        <p>
          Your Mattermost integration will post notifications to the
          team/channel you specify whenever a teammate makes an access request.
          In addition, the integration will match Mattermost and Teleport emails
          for the suggested reviewers defined in the request and alert them as
          well.
        </p>
        <p>
          You will first need to register a Mattermost bot as depicted{' '}
          <Link
            target="_blank"
            href="https://goteleport.com/docs/access-controls/access-request-plugins/ssh-approval-mattermost/#step-58-register-a-mattermost-bot"
          >
            in this step
          </Link>
          . And then invite the bot to the channel that you want the integration
          to post access requests to.
        </p>
      </Text>
    ),
    FormMixin: () => {
      const [url, setUrl] = useState('');
      const [token, setToken] = useState('');
      const [team, setTeam] = useState('');
      const [channel, setChannel] = useState('');
      const [email, setEmail] = useState('');
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
            label="Mattermost Team Name"
            name="team" // must be the same name as expected by the backend as form value
            rule={requiredField('Team Name Required')}
            value={team}
            onChange={e => setTeam(e.target.value)}
            placeholder="team-name"
            toolTipContent="The name of your Mattermost workspace"
            mb={3}
          />
          <FieldInput
            width="500px"
            label="Mattermost Channel Name"
            name="channel" // must be the same name as expected by the backend as form value
            rule={requiredField('Channel Name Required')}
            value={channel}
            onChange={e => setChannel(e.target.value)}
            placeholder="channel-name"
            toolTipContent="Name of the channel to post Access Requests to"
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
             a direct message when the plugin receives an Access Request event`}
            mb={3}
          />
        </>
      );
    },
    NextSteps: () => {
      return (
        <Text>
          <p>
            For help with configuring roles for Access Requests, consult the{' '}
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
    hosted: false,
  },
];

export const pluginMap = Object.fromEntries(plugins.map(p => [p.type, p]));

const InputIconContainer = styled.div`
  position: relative;
`;

const StyledHashtagIcon = styled(Icons.Hashtag)`
  display: inline;
  font-size: 14px;
  padding: 0 6px;
  position: absolute;
  top: 43px;
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
    case 'jamf':
      return IntegrationEnrollKind.Jamf;
    case 'opsgenie':
      return IntegrationEnrollKind.OpsGenie;
    default:
      return IntegrationEnrollKind.Unspecified;
  }
}
