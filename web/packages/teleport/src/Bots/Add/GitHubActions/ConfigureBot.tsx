/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState } from 'react';
import styled from 'styled-components';

import { H2, Text } from 'design';
import { Alert } from 'design/Alert';
import Box from 'design/Box';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';

import { LabelsInput } from 'teleport/components/LabelsInput';
import { getBot } from 'teleport/services/bot';
import useTeleport from 'teleport/useTeleport';

import { FlowButtons } from '../Shared/FlowButtons';
import { FlowStepProps } from '../Shared/GuidedFlow';
import { useGitHubFlow } from './useGitHubFlow';

export function ConfigureBot({ nextStep, prevStep }: FlowStepProps) {
  const [missingLabels, setMissingLabels] = useState(false);
  const [alreadyExistErr, setAlreadyExistErr] = useState(false);

  const { createBotRequest, setCreateBotRequest } = useGitHubFlow();
  const { attempt, run } = useAttempt();
  const isLoading = attempt.status === 'processing';

  const ctx = useTeleport();
  const hasAccess =
    ctx.storeUser.getRoleAccess().create &&
    ctx.storeUser.getTokenAccess().create &&
    ctx.storeUser.getBotsAccess().create;

  async function handleNext(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    if (
      createBotRequest.labels.length < 1 ||
      createBotRequest.labels[0].name === ''
    ) {
      setMissingLabels(true);
      return;
    }

    // check if a bot with that name already exist
    run(async () => {
      const bot = await getBot(createBotRequest.botName);
      if (bot === null) {
        nextStep();
        return;
      }
      setAlreadyExistErr(true);
    });
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
      <Box maxWidth="1080px">
        <Text>
          GitHub Actions is a popular CI/CD platform that works as a part of the
          larger GitHub ecosystem. Teleport Machine ID allows GitHub Actions to
          securely interact with Teleport protected resources without the need
          for long-lived credentials. Through this integration, Teleport will
          create a bot-specific role that scopes its permissions in your
          Teleport instance to the necessary resources and provide inputs for
          your GitHub Actions YAML configuration.
        </Text>
      </Box>
      <Text my="3">
        Teleport supports secure joining on both GitHub-hosted and self-hosted
        GitHub Actions runners as well as GitHub Enterprise Server.
      </Text>

      <H2 mb="2">Step 1: Scope the Permissions for Your Bot</H2>
      <Validation>
        {({ validator }) => (
          <>
            <Box mb="2">
              <Text>
                These first fields will enable Teleport to scope access to only
                what is needed by your GitHub Actions workflow.
              </Text>
            </Box>

            <FormItem>
              <Text>Create a Name for Your Bot Integration*</Text>
              <FieldInput
                rule={requireValidBotName}
                mb={3}
                label=" "
                placeholder="github-actions-cd"
                value={createBotRequest.botName}
                onChange={e =>
                  setCreateBotRequest({
                    ...createBotRequest,
                    botName: e.target.value,
                  })
                }
                css={`
                  margin-bottom: 0;
                `}
              />
              <Text color="text.slightlyMuted" fontSize="small">
                Allowed characters: A-z, 0-9, and -.+
              </Text>
            </FormItem>

            <Box mb="4">
              {missingLabels && (
                <Text mt="1" color="error.main">
                  At least one label is required
                </Text>
              )}
              <LabelsInput
                labels={createBotRequest.labels}
                setLabels={labels =>
                  setCreateBotRequest({ ...createBotRequest, labels: labels })
                }
                disableBtns={isLoading}
                inputWidth={350}
                required={true}
                labelKey={{
                  fieldName: 'Label for Resources the User Can Access',
                  placeholder: 'label key',
                }}
              />
            </Box>
            <FormItem>
              <Text>
                SSH User that Your Bot User Can Access{' '}
                <Text
                  style={{ display: 'inline' }}
                  fontWeight="lighter"
                  fontSize="1"
                >
                  (required field)
                </Text>
              </Text>
              <FieldInput
                mb={3}
                placeholder="ubuntu"
                value={createBotRequest.login}
                onChange={e =>
                  setCreateBotRequest({
                    ...createBotRequest,
                    login: e.target.value,
                  })
                }
                rule={requiredField('SSH user is required')}
              />
            </FormItem>

            {attempt.status === 'failed' && <Alert>{attempt.statusText}</Alert>}
            {alreadyExistErr && (
              <Alert>
                A bot with this name already exist, please use a different name.
              </Alert>
            )}

            <FlowButtons
              isFirstStep={true}
              nextStep={() => handleNext(validator)}
              prevStep={prevStep}
              nextButton={{
                disabled: !hasAccess || isLoading,
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

const validBotNameRegExp = new RegExp('^[0-9A-Za-z./+-]*$');

const requireValidBotName = (value: string) => () => {
  if (!value || !value.trim()) {
    return { valid: false, message: 'Name for the Bot Workflow is required' };
  }

  if (!validBotNameRegExp.test(value)) {
    return {
      valid: false,
      message:
        'Special characters are not allowed in the workflow name, please use name composed only from characters, hyphens, dots, and plus signs',
    };
  }

  return { valid: true };
};
