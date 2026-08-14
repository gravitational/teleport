import { useState } from 'react';
import { Link as InternalLink } from 'react-router';

import {
  Box,
  ButtonPrimary,
  ButtonSecondary,
  Link as ExternalLink,
  Flex,
  Mark,
  Text,
} from 'design';
import FieldInput from 'shared/components/FieldInput';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';

import ecfg from 'e-teleport/config';
import cfg from 'teleport/config';
import { StyledBox } from 'teleport/Discover/Shared';

import { Header } from '../../Shared';
import { Props } from '../GitHub';
import { CreatingDialog } from './CreatingDialog';

export function CreateIntegration({
  gitHubOrgName,
  onGitHubOrgNameChange,
  nextStep,
  emitEvent,
}: Props) {
  const [clientId, setClientId] = useState('');
  const [clientSecret, setClientSecret] = useState('');
  const [showOAuthSteps, setShowOAuthSteps] = useState(false);
  const [creating, setCreating] = useState(false);

  function handleStartOAuthSetup(orgNameValidator: Validator) {
    if (!orgNameValidator.validate()) {
      return;
    }

    setShowOAuthSteps(true);
  }

  function onEdit() {
    setShowOAuthSteps(false);
  }

  async function createResources(oAuthInputValidator: Validator) {
    if (!oAuthInputValidator.validate()) {
      return;
    }

    setCreating(true);
  }

  const disabled = !clientId || !clientSecret;

  return (
    <>
      <Box mb={3} pt={3}>
        <Header header="GitHub Repository Access" />
        <Text>
          Teleport can proxy Git commands and use short-lived SSH certificates
          to authenticate GitHub organizations.{' '}
          <ExternalLink
            target="_blank"
            href="https://goteleport.com/docs/admin-guides/management/guides/github-integration/"
          >
            Learn more
          </ExternalLink>
          .
        </Text>
      </Box>
      <StyledBox mb={5}>
        <Text bold>Step 1</Text>
        Create a GitHub Integration
        <Validation>
          {({ validator: orgNameValidator }) => (
            <>
              <FieldInput
                width="500px"
                label="GitHub Organization Name"
                rule={requiredField('GitHub organization name required')}
                value={gitHubOrgName}
                onChange={e => onGitHubOrgNameChange(e.target.value)}
                placeholder="github-org-name"
                toolTipContent="This name will be used as a Teleport resource name for both a GitHub integration and a Git server"
                disabled={showOAuthSteps}
                mt={2}
              />
              {showOAuthSteps ? (
                <ButtonSecondary
                  width="200px"
                  type="submit"
                  onClick={() => onEdit()}
                >
                  Edit
                </ButtonSecondary>
              ) : (
                <ButtonSecondary
                  width="280px"
                  type="submit"
                  onClick={() => handleStartOAuthSetup(orgNameValidator)}
                  disabled={!gitHubOrgName}
                >
                  Start Configuring an OAuth App
                </ButtonSecondary>
              )}
            </>
          )}
        </Validation>
      </StyledBox>

      <Validation>
        {({ validator: oAuthInputValidator }) => (
          <>
            {showOAuthSteps && (
              <>
                <StyledBox mb={5}>
                  <Box>
                    <Text bold>Step 2</Text>
                    Configure a GitHub OAuth app, which Teleport will use to
                    retrieve login information from your user's browsers. Go to
                    your organization's{' '}
                    <Mark>{'Developer settings > OAuth Apps'}</Mark> settings
                    page and click on{' '}
                    <ExternalLink
                      href={`https://github.com/organizations/${gitHubOrgName}/settings/applications/new`}
                      target="_blank"
                    >
                      New OAuth app
                    </ExternalLink>
                    .
                  </Box>
                  <Box mt={2} mb={2}>
                    Copy and paste the below values to its respective fields in
                    GitHub:
                    <TextSelectCopyMulti
                      bash={false}
                      lines={[
                        {
                          text: 'teleport-github-integration',
                          comment: 'Application name (can be any name):',
                        },
                        { text: `${cfg.baseUrl}`, comment: 'Homepage URL:' },
                        {
                          text: `${cfg.baseUrl}/web/github/integration/callback`,
                          comment: 'Authorization callback URL:',
                        },
                      ]}
                    />
                  </Box>
                  After filling in the fields, click{' '}
                  <Mark>Register application</Mark>
                </StyledBox>

                <StyledBox>
                  <Box mb={2}>
                    <Text bold>Step 3</Text>
                    Copy and paste the generated fields after registering
                    application:
                  </Box>
                  <FieldInput
                    width="500px"
                    label="Client ID"
                    rule={requiredField('Client ID required')}
                    value={clientId}
                    onChange={e => setClientId(e.target.value)}
                    placeholder="1234567a1ab2ab12345a"
                    toolTipContent="Copy and paste the generated Client ID"
                  />
                  <FieldInput
                    width="500px"
                    label="Client Secret"
                    type="password"
                    rule={requiredField('Client Secret required')}
                    value={clientSecret}
                    onChange={e => setClientSecret(e.target.value)}
                    placeholder="12a3b45c6d123456aaa...1a2b3456"
                    toolTipContent="Copy and paste the generated client secret after clicking on 'Generate a new client secret'"
                  />
                </StyledBox>
              </>
            )}
            <Flex mt={4} mb={5} gap={3}>
              <ButtonPrimary
                onClick={() => createResources(oAuthInputValidator)}
                disabled={disabled}
              >
                Next
              </ButtonPrimary>

              <ButtonSecondary
                as={InternalLink}
                to={ecfg.oss.getIntegrationEnrollRoute()}
              >
                Back
              </ButtonSecondary>
            </Flex>
          </>
        )}
      </Validation>
      {creating && (
        <CreatingDialog
          next={nextStep}
          cancel={() => setCreating(false)}
          gitHubOrgName={gitHubOrgName}
          clientId={clientId}
          clientSecret={clientSecret}
          emitEvent={emitEvent}
        />
      )}
    </>
  );
}
