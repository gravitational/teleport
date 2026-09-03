import { useState } from 'react';

import { Box, Text } from 'design';
import * as Icons from 'design/Icon';
import { RadioGroup } from 'design/RadioGroup';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';

import { StyledBox } from 'e-teleport/Integrations/Shared';
import {
  EnrollMethodType,
  PluginConfigBase,
} from 'e-teleport/services/plugins';

import { FormDataField } from './types';

export function FormMixin() {
  const [channel, setChannel] = useState('');

  const [name, setName] = useState('');

  // Slack plugin can be enrolled either via
  // static credentials or via OAuth 2.0 flow.
  const [enrollMethod, setEnrollMethod] = useState<EnrollMethodType>('oauth');

  // Fields for static credentials enrollment only.
  const [botToken, setBotToken] = useState('');

  return (
    <Box width="800px">
      <StyledBox header="Select Enrollment Method" mt={4}>
        <RadioGroup
          size="large"
          name={PluginConfigBase.EnrollMethod}
          value={enrollMethod}
          onChange={(v: `${EnrollMethodType}`) => {
            setEnrollMethod(v);

            // Clear selections conditional on static enrollment method.
            setBotToken('');
          }}
          autoFocus={true}
          options={[
            {
              value: 'oauth',
              label: 'Teleport-Managed App',
              helperText: (
                <Text>
                  Connect to Slack using Teleport Cloud's managed Slack app.
                  <br />
                  Teleport Cloud automatically manages this for easy enrollment.
                  For users deploying multiple plugin instances or wanting
                  stronger cluster isolation, choose the{' '}
                  <Text as="span" bold>
                    Bring Your Own App
                  </Text>{' '}
                  enrollment method instead.
                </Text>
              ),
            },
            {
              value: 'static',
              label: 'Bring Your Own App',
              helperText: (
                <Text>
                  Connect to Slack by bringing your own Slack app.
                  <br />
                  Recommended if you plan on deploying multiple plugin instances
                  to the same Slack workspace.
                </Text>
              ),
            },
          ]}
        ></RadioGroup>
      </StyledBox>
      <StyledBox header="Configuration Fields" mt={4}>
        <FieldInput
          width="500px"
          label="Default Channel"
          name={FormDataField.FallbackChannel}
          rule={requiredField('Default channel must be specified')}
          value={channel}
          onChange={e => setChannel(e.target.value)}
          placeholder="access-requests"
          toolTipContent="The default channel will receive all notifications about access requests. Request notifications will also be sent directly to assigned reviewers (if any)."
          icon={Icons.Hashtag}
          mb={3}
        />

        <FieldInput
          width="500px"
          label="Name"
          name={PluginConfigBase.Name}
          value={name}
          onChange={e => setName(e.target.value)}
          placeholder="slack-default"
          toolTipContent={
            <Text>
              The name of the Slack plugin instance. If unspecified, it will be
              set to{' '}
              <Text as="span" bold>
                slack-default
              </Text>
              .
            </Text>
          }
          mb={3}
        />

        {enrollMethod === 'static' && (
          <FieldInput
            width="500px"
            label="Bot API Token"
            name={FormDataField.BotToken}
            rule={requiredField('Slack Bot token must be specified')}
            value={botToken}
            type="password"
            onChange={e => setBotToken(e.target.value)}
            placeholder="xoxb-your-bot-token"
            toolTipContent="The Slack Bot token provides the plugin permissions to read Slack user info and write to channels."
            mb={0}
          />
        )}
      </StyledBox>
    </Box>
  );
}
