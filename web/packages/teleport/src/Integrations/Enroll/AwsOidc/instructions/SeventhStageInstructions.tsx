/**
 * Copyright 2023 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React, { useState, useEffect } from 'react';
import { useLocation } from 'react-router';
import { Link } from 'react-router-dom';
import { Danger } from 'design/Alert';
import { CircleCheck } from 'design/Icon';
import { ButtonPrimary, ButtonSecondary, Text, Flex, Box } from 'design';
import Dialog, {
  DialogHeader,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  requiredField,
  requiredRoleArn,
} from 'shared/components/Validation/rules';

import {
  Integration,
  IntegrationKind,
  integrationService,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';
import { DiscoverUrlLocationState } from 'teleport/Discover/useDiscover';
import { IntegrationEnrollEvent } from 'teleport/services/userEvent';

import { InstructionsContainer, PreviousStepProps } from './common';

type EmitEvent = (event: IntegrationEnrollEvent) => void;

export function SeventhStageInstructions(
  props: PreviousStepProps & { emitEvent: EmitEvent }
) {
  const { attempt, setAttempt } = useAttempt('');
  const [createdIntegration, setCreatedIntegration] = useState<Integration>();
  const [roleArn, setRoleArn] = useState(props.awsOidc.roleArn);
  const [name, setName] = useState(props.awsOidc.integrationName);

  function handleSubmit(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    setAttempt({ status: 'processing' });
    integrationService
      .createIntegration({
        name,
        subKind: IntegrationKind.AwsOidc,
        awsoidc: { roleArn },
      })
      .then(setCreatedIntegration)
      .catch((err: Error) =>
        setAttempt({ status: 'failed', statusText: err.message })
      );
  }

  return (
    <InstructionsContainer>
      {attempt.status === 'failed' && (
        <Danger mb={5}>{attempt.statusText}</Danger>
      )}
      <Text>From the list of roles, select the role you just created</Text>

      <Validation>
        {({ validator }) => (
          <>
            <Text mt={5}>Copy the role ARN and paste it below</Text>
            <Box mt={2}>
              <FieldInput
                autoFocus
                label="Role ARN"
                onChange={e => setRoleArn(e.target.value)}
                value={roleArn}
                placeholder="Role ARN"
                rule={requiredRoleArn}
                toolTipContent={
                  <Text>
                    Role ARN can be found in the format: <br />
                    {`arn:aws:iam::<ACCOUNT_ID>:role/<ROLE_NAME>`}
                  </Text>
                }
              />
            </Box>
            <Text mt={5}>Give this AWS integration a name</Text>
            <Box mt={2}>
              <FieldInput
                label="Name"
                onChange={e => setName(e.target.value)}
                value={name}
                placeholder="Integration name"
                rule={requiredField('Name is required')}
              />
            </Box>
            <Box mt={5}>
              <ButtonPrimary
                onClick={() => handleSubmit(validator)}
                disabled={attempt.status === 'processing'}
              >
                Next
              </ButtonPrimary>
              <ButtonSecondary
                ml={3}
                onClick={() =>
                  props.onPrev({
                    ...props.awsOidc,
                    roleArn,
                    integrationName: name,
                  })
                }
                disabled={attempt.status === 'processing'}
              >
                Back
              </ButtonSecondary>
            </Box>
          </>
        )}
      </Validation>
      {createdIntegration && (
        <SuccessfullyAddedIntegrationDialog
          integration={createdIntegration}
          emitEvent={props.emitEvent}
        />
      )}
    </InstructionsContainer>
  );
}

export function SuccessfullyAddedIntegrationDialog({
  integration,
  emitEvent,
}: {
  integration: Integration;
  emitEvent: EmitEvent;
}) {
  const location = useLocation<DiscoverUrlLocationState>();

  useEffect(() => {
    if (location.state?.discover) {
      return;
    }
    emitEvent(IntegrationEnrollEvent.Complete);
    // Only send event once on init.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={true}
      onClose={close}
      open={true}
    >
      <DialogHeader css={{ margin: '0 auto' }}>
        <CircleCheck mb={4} fontSize={60} color="success" />
      </DialogHeader>
      <DialogContent>
        <Text textAlign="center">
          AWS integration "{integration.name}" successfully added
        </Text>
      </DialogContent>
      <DialogFooter css={{ margin: '0 auto' }}>
        {location.state?.discover ? (
          <ButtonPrimary
            size="large"
            as={Link}
            to={{
              pathname: cfg.routes.discover,
              state: {
                integration,
                discover: location.state.discover,
              },
            }}
          >
            Begin RDS Enrollment
          </ButtonPrimary>
        ) : (
          <Flex gap="3">
            <ButtonPrimary as={Link} to={cfg.routes.integrations} size="large">
              Go to Integration List
            </ButtonPrimary>
            <ButtonSecondary
              as={Link}
              to={cfg.getIntegrationEnrollRoute(null)}
              size="large"
            >
              Add Another Integration
            </ButtonSecondary>
          </Flex>
        )}
      </DialogFooter>
    </Dialog>
  );
}
