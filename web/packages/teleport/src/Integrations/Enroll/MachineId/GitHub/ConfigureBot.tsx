import React, { useState } from 'react';
import styled from 'styled-components';

import Box from 'design/Box';
import Validation, { Validator } from 'shared/components/Validation';

import Text from 'design/Text';

import FieldInput from 'shared/components/FieldInput';

import { requiredField } from 'shared/components/Validation/rules';

import Alert from 'design/Alert';

import useTeleport from 'teleport/useTeleport';

import { FlowStepProps } from '../shared/GuidedFlow';
import { FlowButtons } from '../shared/FlowButtons';

import { LabelsInput } from './LabelsInput';

import { useGitHubFlow } from './useGitHubFlow';

export function ConfigureBot({ nextStep, prevStep }: FlowStepProps) {
  const [missingLabels, setMissingLabels] = useState(false);

  const { botConfig, setBotConfig } = useGitHubFlow();

  const ctx = useTeleport();
  const hasAccess =
    ctx.storeUser.getRoleAccess().create &&
    ctx.storeUser.getTokenAccess().create &&
    ctx.storeUser.getBotsAccess().create;

  function handleNext(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    if (botConfig.labels.length < 1 || botConfig.labels[0].name === '') {
      setMissingLabels(true);
      return;
    }

    nextStep();
  }

  return (
    <Box>
      {!hasAccess && (
        <Alert kind="danger">
          <Text>
            Insufficient permissions. In order to create a bot, you need
            permissions to create roles, bots and join tokens.
          </Text>
        </Alert>
      )}
      <Text>
        GitHub Actions is a popular CI/CD platform that works as a part of the
        larger GitHub ecosystem. Teleport Machine ID allows GitHub Actions to
        securely interact with Teleport protected resources without the need for
        long-lived credentials. Through this integration, Teleport will create a
        bot-specific role that scopes its permissions in your Teleport instance
        to the necessary resources and provide inputs for your GitHub Actions
        YAML configuration.
      </Text>
      <Text my="3">
        Teleport supports secure joining on both GitHub-hosted and self-hosted
        GitHub Actions runners as well as GitHub Enterprise Server.
      </Text>

      <Text bold fontSize={4} mb="3">
        Step 1: Scope the Permissions for Your Machine User
      </Text>
      <Validation>
        {({ validator }) => (
          <>
            <Box mb="4">
              <Text>
                These first fields will enable Teleport to scope access to only
                what is needed by your GitHub Actions workflow.
              </Text>
              {missingLabels && (
                <Text mt="1" color="error.main">
                  At least one label is required
                </Text>
              )}
              <LabelsInput
                labels={botConfig.labels}
                setLabels={labels =>
                  setBotConfig({ ...botConfig, labels: labels })
                }
                disableBtns={false} // TODO
              />
            </Box>
            <FormItem>
              <Text>
                SSH User that Your Machine User Can Access{' '}
                <Text
                  style={{ display: 'inline' }}
                  fontWeight="lighter"
                  fontSize="1"
                >
                  (optional)
                </Text>
              </Text>
              <FieldInput
                mb={3}
                placeholder="ex. ubuntu"
                value={botConfig.login}
                onChange={e =>
                  setBotConfig({ ...botConfig, login: e.target.value })
                }
              />
            </FormItem>

            <FormItem>
              <Text>Create a Name for Your Machine User *</Text>
              <FieldInput
                rule={requiredField('Name for Machine User is required')}
                mb={3}
                placeholder="ex. github-actions-cd"
                value={botConfig.botName}
                onChange={e =>
                  setBotConfig({ ...botConfig, botName: e.target.value })
                }
              />
            </FormItem>

            <FlowButtons
              isFirst={true}
              nextStep={() => handleNext(validator)}
              prevStep={prevStep}
              nextButton={{
                disabled: !hasAccess,
              }}
            />
          </>
        )}
      </Validation>
    </Box>
  );
}

const FormItem = styled(Box)`
  margin-bottom: ${props => props.theme.space[4]}px;
  max-width: 500px;
`;
