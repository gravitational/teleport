import React, { useState } from 'react';
import styled from 'styled-components'
import ReactSelect from 'react-select';

import Box from "design/Box";
import useTeleport from 'teleport/useTeleport';
import Text from 'design/Text';
import Select, { Option, StyledSelect } from 'shared/components/Select';
import { GitHubBotConfig } from 'teleport/services/bot/types';
import Input from 'design/Input';
import { ButtonPrimary } from 'design/Button';
import useAttempt from 'shared/hooks/useAttemptNext';
import Validation, { Validator } from 'shared/components/Validation';
import { FlowStepProps } from '../Flow/Flow';
import { FlowButtons } from '../Flow/FlowButtons';
import Flex from 'design/Flex';

type RefTypeOption = Option<'branch' | 'tag'>;
const refTypeOptions: RefTypeOption[] = [
  {
    label: 'Branch',
    value: 'branch',
  },
  {
    label: 'Tag',
    value: 'tag',
  }
]

export function ConnectGitHub({ nextStep, prevStep }: FlowStepProps) {
  // For now, I'm keeping everything here to avoid refactoring too much
  // in case design doesn't end up as I assume it will.
  // TODOs:
  // - Loading and failed states
  // - Permission checking
  // - General styling
  // - A previous way to select type (join method) of bot. I'm assuming its GitHub here
  // - Form validation

  const ctx = useTeleport();

  const [workflowName, setWorkflowName] = useState<string>("");
  const [ref, setRef] = useState<string>('main');
  const [refType, setRefType] = useState<RefTypeOption>(refTypeOptions[0]);
  const [repoAddress, setRepoAddress] = useState<string>("");

  // const { run, attempt } = useAttempt()

  // function handleSubmit(validator: Validator) {
  //   if (!validator.validate()) {
  //     return;
  //   }

  //   const repoUrl = `https://${repoAddress}`
  //   const { repository, repositoryOwner } = parseGitHubUrl(repoUrl)
  //   // validator should avoid getting into this
  //   if (!repository || !repositoryOwner) {
  //     // invalid repo url
  //   }

  //   ctx.botService.createGitHubBot({
  //     actor: actorName,
  //     environment: 'todo',
  //     name: botName,
  //     ref: ref,
  //     refType: refType.value,
  //     repository: repository,
  //     repositoryOwner: repositoryOwner,
  //     subject: 'todo',
  //     workflow: 'todo',
  //     roles: [],
  //   })
  // }

  function handleRepoAddress(event: React.FormEvent<HTMLInputElement>) {
    console.log(event)
    // const url = event.target?.value
  }

  return (
    <Box as="form" onSubmit={(e) => {
      e.preventDefault()
    }}>
      {/* {attempt.status === 'processing' && (
        <Box>Loading...</Box>
      )}
      {attempt.status === 'success' && */}
      {/* TODO: validate input */}
      <Text>
        GitHub Actions is a popular CI/CD platform that works as a part of the larger GitHub ecosystem. Teleport Machine ID allows GitHub Actions to securely interact with Teleport protected resources without the need for long-lived credentials. Through this integration, Teleport will create a bot-specific role that scopes its permissions in your Teleport instance to the necessary resources and provide inputs for your GitHub Actions YAML configuration.
      </Text>
      <Text mt="3">
        Teleport supports secure joining on both GitHub-hosted and self-hosted GitHub Actions runners as well as GitHub Enterprise Server.
      </Text>

      <Text bold fontSize="3" mt="3">
        Examples of How to Combine GitHub Actions and Machine ID (TODO)
      </Text>

      <Validation>
        {({ validator }) => (
          <Box mt="3">
            <FormItem>
              <Text>Full URL to Your Repository*</Text>
              <Input
                mb={3}
                label="Full URL to Your Repository *"
                placeholder="github.com/gravitational/teleport"
                value={repoAddress}
                onChange={(e) => setRepoAddress(e.target.value)}
              />
            </FormItem>

            <FormItem>
              <Text>Name of the GitHub Actions Workflow *</Text>
              <Input
                mb={3}
                // TODO disabled
                label="Name of the GitHub Actions Workflow"
                placeholder="teleport-auth"
                value={workflowName}
                onChange={(e) => setWorkflowName(e.target.value)}
              />
            </FormItem>

            <FormItem>
              <Flex>
                <Box>
                  <Text ml="1">Git Ref</Text>
                  <Input
                    mb={3}
                    label="Git Ref"
                    placeholder="main"
                    value={ref}
                    onChange={(e) => setRef(e.target.value)}
                    style={{ borderRadius: '4px 0 0 4px' }}
                  // TODO use radius from theme
                  />
                </Box>
                <Box width="100px">
                  <Text ml="1">Ref Type</Text>
                  <RefTypeSelect>
                    <ReactSelect
                      isMulti={false}
                      value={refType}
                      onChange={(o: RefTypeOption) => setRefType(o)}
                      options={refTypeOptions}
                      menuPlacement="auto"
                      className="react-select-container"
                      classNamePrefix="react-select"
                    />
                  </RefTypeSelect>
                </Box>
              </Flex>
            </FormItem>
            <FlowButtons isFirst={false} isLast={false} nextStep={nextStep} prevStep={prevStep} />
          </Box>
        )}
      </Validation>

      {/* } */}
    </Box>
  )
}

const RefTypeSelect = styled(StyledSelect)`
.react-select__control {
  border-radius: 0 4px 4px 0;
`

const FormItem = styled(Box)`
  margin-bottom: ${props => props.theme.space[4]}px;
`
/**
 * parseGitHubUrl parses a URL for a github repository. It throws if parsing the URL fails
 * or the URL doesn't look like a github repository URL
 * @param {string} repoUrl - repository URL with protocol
 * @returns {repository: string, repositoryOwner: tring} - object containing repository and repository owner
 */
export function parseGitHubUrl(repoUrl: string): { repository: string, repositoryOwner: string } {
  const u = new URL(repoUrl)
  const paths = u.pathname.split('/')
  // expected length is 3, since pathname starts with a /, so paths[0] should be empty
  if (paths.length < 3) {
    throw new Error("URL expected to be in the format <host>/<owner>/<repository>")
  }
  return {
    repositoryOwner: paths[1],
    repository: paths[2],
  }
}