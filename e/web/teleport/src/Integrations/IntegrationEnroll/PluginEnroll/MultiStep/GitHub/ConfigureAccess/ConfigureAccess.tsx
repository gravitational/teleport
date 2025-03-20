import { useState } from 'react';

import {
  Alert,
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Flex,
  Mark,
  Text,
} from 'design';
import FieldInput from 'shared/components/FieldInput';
import TextEditor from 'shared/components/TextEditor';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';
import { getErrMessage } from 'shared/utils/errorType';

import { StyledBox } from 'teleport/Discover/Shared';
import ResourceService from 'teleport/services/resources';
import {
  IntegrationEnrollEvent,
  IntegrationEnrollStatusCode,
} from 'teleport/services/userEvent';

import { Header } from '../../Shared';
import { getIntegrationName } from '../getIntegrationName';
import { Props } from '../GitHub';
import { FinishDialog } from './FinishDialog';

export function ConfigureAccess({ gitHubOrgName, emitEvent }: Props) {
  const [skipStep, setSkipStep] = useState(false);

  const [roleName, setRoleName] = useState(`github-${gitHubOrgName}`);
  const roleYaml = `kind: role
metadata:
  name: ${roleName}
spec:
  allow:
    github_permissions:
    - orgs:
      - ${gitHubOrgName}
version: v7
`;

  const [createRoleAttempt, createRole] = useAsync(async () => {
    const resourceService = new ResourceService();
    try {
      await resourceService.createRole(roleYaml);
      emitEvent({ status: { code: IntegrationEnrollStatusCode.Success } });
      emitEvent({ event: IntegrationEnrollEvent.Complete });
    } catch (err) {
      emitEvent({
        status: {
          code: IntegrationEnrollStatusCode.Error,
          error: getErrMessage(err),
        },
      });
      throw err;
    }
  });

  async function onNext(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    await createRole();
  }

  function onSkip() {
    emitEvent({ status: { code: IntegrationEnrollStatusCode.Skipped } });
    setSkipStep(true);
  }

  const createdRole = createRoleAttempt.status === 'success';
  const disableInput = createdRole || createRoleAttempt.status === 'processing';

  return (
    <>
      <Box mb={3} pt={3}>
        <Header header="Configure access to Git server (Optional)" />
        <Text>
          Create a Teleport role that will grant access to Git server{' '}
          <Mark>{getIntegrationName(gitHubOrgName)}</Mark>.
        </Text>
      </Box>

      {createRoleAttempt.status === 'error' && (
        <Alert kind="danger">{createRoleAttempt.statusText}</Alert>
      )}

      <Validation>
        {({ validator }) => (
          <>
            <StyledBox>
              <FieldInput
                width="500px"
                label="Teleport Role Name"
                rule={requiredField('Teleport role name required')}
                value={roleName}
                onChange={e => setRoleName(e.target.value)}
                placeholder={`github-${gitHubOrgName}`}
                toolTipContent="Name of the Teleport role that will give users access to this Git server."
                autoFocus
                disabled={disableInput}
              />
              Preview of the role that will be created:
              <Flex minHeight="190px">
                <TextEditor
                  key={roleName}
                  readOnly={true}
                  bg="levels.deep"
                  data={[{ content: roleYaml, type: 'yaml' }]}
                />
              </Flex>
            </StyledBox>

            <Flex mt={4} mb={5} gap={3}>
              <ButtonPrimary
                onClick={() => onNext(validator)}
                disabled={
                  createRoleAttempt.status === 'processing' || !roleName
                }
              >
                Finish
              </ButtonPrimary>
              <ButtonSecondary
                onClick={onSkip}
                disabled={createRoleAttempt.status === 'processing'}
              >
                Skip
              </ButtonSecondary>
            </Flex>
          </>
        )}
      </Validation>

      {(createdRole || skipStep) && (
        <FinishDialog
          role={{ created: createdRole, name: roleName }}
          gitHubOrgName={gitHubOrgName}
        />
      )}
    </>
  );
}
