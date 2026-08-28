import styled from 'styled-components';

import { Box, ButtonPrimaryBorder, Card, Flex, H1, H2, Text } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import { useAccessListManagementContext } from 'e-teleport/AccessListManagement/AccessListManagementContext';
import { useCreateAccessList } from 'e-teleport/AccessListManagement/CreateAccessList/CreateAccessListContextProvider';
import { Finished } from 'e-teleport/AccessListManagement/CreateAccessList/Finished';
import { definableResourceAccessFields } from 'e-teleport/AccessListManagement/GuideEditor/Preset/role/listaccess';
import { StepButtons } from 'e-teleport/AccessListManagement/GuideEditor/Shared';
import { TerraformDeploymentCreate } from 'e-teleport/AccessListManagement/GuideEditor/Terraform/TerraformDeploymentCreate';
import { Prompt } from 'teleport/components/Router';

import { Summary } from '../Summary/Summary';
import { cancelPrompt } from '../types';
import { ErrorCreatingAccessListDialog } from './ErrorCreatingAccessListDialog';

export function DeploymentMethods() {
  const {
    spec,
    onCreate,
    createAttempt,
    setCreateAttempt,
    members,
    owners,
    requiresCleanup,
    dismissCleanupError,
  } = useCreateAccessList();

  const { guideEditor } = useAccessListManagementContext();
  const {
    terraform,
    deploymentView,
    setDeploymentView,
    preset,
    standardRoleState,
    awsIcRoleState,
    definedAccess,
  } = guideEditor;

  if (deploymentView === 'finished') {
    return <Finished />;
  }

  if (deploymentView === 'terraform') {
    return <TerraformDeploymentCreate onPrev={() => setDeploymentView('')} />;
  }

  async function handleCreate(validator: Validator) {
    const created = await onCreate(validator);
    if (created) {
      setDeploymentView('finished');
      terraform.updateSidePanel(0);
    }
  }

  const definedAccessFields = definableResourceAccessFields.filter(field =>
    definedAccess(field)
  );

  return (
    <>
      <Box p={8} pt={5} width="100%">
        <Card p={4} maxWidth={'640px'} minWidth={'580px'} mx="auto">
          <Box mb={4} textAlign="center">
            <H1>Choose Deployment Method</H1>
            <Text>
              <Text as="span" bold>
                {spec.title}
              </Text>{' '}
              is configured and ready for deployment.
            </Text>
          </Box>
          <Flex>
            <Pane
              title="Manage via Terraform"
              text={
                <>
                  <Text>Fully managed via Terraform.</Text>
                  <Text>No audit support.</Text>
                </>
              }
              button={
                <PaneButton
                  disabled={createAttempt.status === 'processing'}
                  onClick={() => {
                    setDeploymentView('terraform');
                  }}
                >
                  Continue via Terraform
                </PaneButton>
              }
            />

            <Divider />

            <Validation>
              {({ validator }) => (
                <>
                  <Pane
                    title={'Manage in Teleport'}
                    text={
                      <>
                        <Text>Managed via Teleport Web.</Text>
                        <Text>Auditing supported.</Text>
                      </>
                    }
                    button={
                      <PaneButton
                        onClick={() => handleCreate(validator)}
                        disabled={createAttempt.status === 'processing'}
                      >
                        Create Access List Now
                      </PaneButton>
                    }
                  />
                  {(createAttempt.status === 'failed' ||
                    requiresCleanup?.error) && (
                    <ErrorCreatingAccessListDialog
                      error={createAttempt.statusText}
                      onCancel={() => {
                        setCreateAttempt({ status: '' });
                        dismissCleanupError();
                      }}
                      retry={() => handleCreate(validator)}
                      requiresCleanup={requiresCleanup}
                    />
                  )}
                </>
              )}
            </Validation>
          </Flex>
        </Card>

        <Flex flexDirection={'column'} alignItems={'center'} mt={5}>
          <Summary
            spec={spec}
            preset={preset}
            members={members}
            owners={owners}
            awsIcRoleConditions={awsIcRoleState.roleConditions}
            standardRoleConditions={standardRoleState.roleConditions}
            definedAccessFields={definedAccessFields}
          />
        </Flex>
      </Box>
      <StepButtons
        hideNextBtn
        disabled={createAttempt.status === 'processing'}
      />
      {deploymentView === '' && <Prompt when message={cancelPrompt} />}
    </>
  );
}

function Pane({
  title,
  text,
  button,
}: {
  title: string;
  text: React.ReactNode;
  button: React.ReactNode;
}) {
  return (
    <Flex p={4} pt={3} flex={1} textAlign="center" flexDirection="column">
      {title && <H2>{title}</H2>}
      <Box mb={2}>{text}</Box>
      {button}
    </Flex>
  );
}

const PaneButton = styled(ButtonPrimaryBorder).attrs({
  width: '100%',
  size: 'large',
})``;

function Divider() {
  const height = '60px';
  return (
    <Flex
      width="48px"
      flexDirection="column"
      alignItems="center"
      justifyContent="center"
      flexShrink={0}
    >
      <Box width="1px" height={height} bg="text.muted" />
      <Text my={2} color="text.muted">
        or
      </Text>
      <Box width="1px" height={height} bg="text.muted" />
    </Flex>
  );
}
