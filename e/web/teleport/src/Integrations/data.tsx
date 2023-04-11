import React, { useState } from 'react';
import styled from 'styled-components';

import { Box, Text } from 'design';
import * as Icons from 'design/Icon';
import slackIcon from 'design/assets/images/icons/slack.svg';
import pagerdutyIcon from 'design/assets/images/icons/pagerduty.svg';
import emailIcon from 'design/assets/images/icons/email.svg';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';

type Permission = {
  title: string;
  description?: string;
};

type CategoryPermissions = {
  category: string;
  permissions: Permission[];
};

type HostedPluginData = {
  hosted: true;

  // For each hosted plugin, these describe additional elements in the enroll page.
  fullName: string;
  Description?: () => JSX.Element;
  FormMixin?: () => JSX.Element;
  permissions?: CategoryPermissions[];
};

export type PluginTypes = 'slack' | 'pagerduty' | 'email';

export type PluginType = {
  type: PluginTypes;
  name: string;
  icon: string;
  url: string;
} & ({ hosted: false } | HostedPluginData);

export const pluginTypes: PluginType[] = [
  {
    type: 'slack',
    name: 'Slack',
    icon: slackIcon,
    url: 'https://github.com/gravitational/teleport-plugins/tree/master/access/slack',
    hosted: true,
    fullName: 'Slack Access Notifications',
    Description: () => (
      <Box>
        <Text>
          <p>
            Your Slack integration will match Slack and Teleport emails for
            Teleport reviewers and alert them whenever a teammate makes an
            access request. To do that, the integration will need you* to
            approve the following permissions:
          </p>
          <p>
            *Please note that if you do not have permissions to add new apps to
            your Slack workspace, you will need to request approval in the next
            step. Once that approval is given, Slackbot will notify you within
            your workspace, and you will be able to connect Slack and Teleport
            by coming back to this view and clicking the “Connect Slack” button
            again.
          </p>
        </Text>
      </Box>
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
            label="Default channel"
            name="fallback_channel"
            rule={requiredField('Fallback channel must be specified')}
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
  // TODO(justinas): add remaining self-hosted plugins
];

export const pluginTypeMap = Object.fromEntries(
  pluginTypes.map(p => [p.type, p])
);

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
