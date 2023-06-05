import React, { useState } from 'react';
import styled from 'styled-components';

import { Text } from 'design';
import CardError from 'design/CardError';
import * as Icons from 'design/Icon';
import slackIcon from 'design/assets/images/icons/slack.svg';
import pagerdutyIcon from 'design/assets/images/icons/pagerduty.svg';
import emailIcon from 'design/assets/images/icons/email.svg';
import jiraIcon from 'design/assets/images/icons/jira.svg';
import discordIcon from 'design/assets/images/icons/discord.svg';
import mattermostIcon from 'design/assets/images/icons/mattermost.svg';
import msteamsIcon from 'design/assets/images/icons/msteams.svg';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import { IntegrationEnrollKind } from 'teleport/services/userEvent';

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

// PluginTypes represents the type of the plugin
// and should be the same value as defined in the backend:
// https://github.com/gravitational/teleport/blob/a410acef01e0023d41c18ca6b0a7b384d738bb32/api/types/plugin.go#L27
export type PluginTypes =
  | 'slack'
  | 'pagerduty'
  | 'email'
  | 'jira'
  | 'discord'
  | 'mattermost'
  | 'msteams'
  | 'opsgenie'
  | 'okta'
  | 'jamf';

export type PluginBase = {
  type: PluginTypes;
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
            name="fallback_channel"
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
    type: 'pagerduty',
    name: 'PagerDuty',
    icon: pagerdutyIcon,
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/pagerduty',
    hosted: false,
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
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/mattermost',
    hosted: false,
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

export function pluginTypeToIntegrationEnrollKind(p: PluginTypes) {
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
