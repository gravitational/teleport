import React, { useState } from 'react';
import styled from 'styled-components'
import ReactSelect from 'react-select';

import Box from "design/Box";
import useTeleport from 'teleport/useTeleport';
import Text from 'design/Text';
import * as Icons from 'design/Icon';
import Select, { Option, StyledSelect } from 'shared/components/Select';
import { GitHubBotConfig } from 'teleport/services/bot/types';
import Input from 'design/Input';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import useAttempt from 'shared/hooks/useAttemptNext';
import Validation, { Validator } from 'shared/components/Validation';
import { FlowStepProps } from '../../Flow/Flow';
import { FlowButtons } from '../../Flow/FlowButtons';
import Flex from 'design/Flex';
import Card from 'design/Card';
import ButtonIcon from 'design/ButtonIcon';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';

type RefType = 'branch' | 'tag'

type RefTypeOption = Option<RefType>;
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

// TODO move type to another file?
type Rule = {
  workflowName: string,
  environment: string,
  ref: string,
  refType: RefTypeOption,
  repoAddress: string,
  actor: string,
}

const defaultRule: Rule = {
  workflowName: '',
  environment: '',
  ref: '',
  refType: { label: 'Branch', value: 'branch' },
  repoAddress: '',
  actor: '',
}

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

  const [rules, setRules] = useState<Rule[]>([defaultRule])

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

  function handleNext(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    // TODO save values in flow-wide state
    nextStep();
  }

  function handleChange(
    index: number,
    field: keyof Rule,
    value: Rule[typeof field]) {
    // const { value } = event.target
    const newRules = [...rules]
    newRules[index] = { ...newRules[index], [field]: value }
    setRules(newRules);
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


      <Validation>
        {({ validator }) => (
          <>
            <Box mt="3">
              <Text bold fontSize={4} mb="3">Step 2: Input Your GitHub Account Info</Text>
              <Text mb="3">These fields will be combined with your machine user’s permissions to create a join token and generate a sample GitHub Actions file.</Text>

              {rules.map((rule, i) =>
              (
                <Card p="4" maxWidth="540px" key={i} mb="4">
                  <Box>
                    <>
                      <Flex alignItems="center" justifyContent="space-between">
                        <Text bold>GitHub Repository Access:</Text>
                        {i > 0 &&
                          <ButtonIcon
                            size={1}
                            title="Remove Rule"
                            onClick={() => setRules(rules.filter((r, index) => index !== i))}
                          >
                            <Icons.Trash size="medium" />
                          </ButtonIcon>
                        }
                      </Flex>
                      <FormItem>
                        <Text mt="3">Full URL to Your Repository*</Text>
                        <FieldInput
                          rule={requireValidRepository}
                          label=" "
                          placeholder="ex. https://github.com/gravitational/teleport"
                          value={rules[i].repoAddress}
                          onChange={(e) => handleChange(i, "repoAddress", e.target.value)}
                        />
                      </FormItem>

                      <FormItem>
                        <Flex>
                          <Box width="100%">
                            <Text>Git Ref <OptionalFieldText /></Text>
                            <Input
                              label="Git Ref"
                              placeholder="main"
                              style={{ borderRadius: '4px 0 0 4px' }}
                              value={rules[i].ref}
                              onChange={(e) => handleChange(i, "ref", e.target.value)}
                            />
                          </Box>
                          <Box minWidth="160px">
                            <Text ml="1">Ref Type</Text>
                            <RefTypeSelect>
                              <ReactSelect
                                isMulti={false}
                                value={rules[i].refType}
                                onChange={(o) => handleChange(i, "refType", o)}
                                options={refTypeOptions}
                                menuPlacement="auto"
                                className="react-select-container"
                                classNamePrefix="react-select"
                              />
                            </RefTypeSelect>
                          </Box>
                        </Flex>
                      </FormItem>

                      <FormItem>
                        <Text>Name of the GitHub Actions Workflow *</Text>
                        <FieldInput
                          // TODO disabled
                          placeholder="ex. cd"
                          rule={requiredField("Workflow name is required")}
                          value={rules[i].workflowName}
                          onChange={(e) => handleChange(i, "workflowName", e.target.value)}
                        />
                      </FormItem>

                      <FormItem>
                        <Text>Environmnet <OptionalFieldText /></Text>
                        <Input
                          // TODO disabled
                          placeholder="ex. development"
                          value={rules[i].environment}
                          onChange={(e) => handleChange(i, "environment", e.target.value)}
                        />
                      </FormItem>

                      <Box>
                        <Text>Restrict to a GitHub User<OptionalFieldText />  </Text>
                        <Input
                          // TODO disabled
                          placeholder="ex. octocat"
                          value={rules[i].actor}
                          onChange={(e) => handleChange(i, "actor", e.target.value)}
                        />
                        <Text fontWeight="lighter" fontSize="1" style={{ fontStyle: "italic" }}>If left blank, any GitHub user can trigger the workflow</Text>
                      </Box>

                    </>
                  </Box>
                </Card>
              )
              )}
              <Box mb="4">
                <ButtonSecondary onClick={() => setRules([...rules, defaultRule])}>+ Add Another Set of Repository Rules</ButtonSecondary>
              </Box>
              <FlowButtons nextStep={() => handleNext(validator)} prevStep={prevStep} />
            </Box>
          </>
        )}
      </Validation>
    </Box >
  )
}

const RefTypeSelect = styled(StyledSelect)`
.react-select__control {
  border-radius: 0 4px 4px 0;
  border-left: none;
}

.react-select__control:hover {
  border: 1px solid rgba(0,0,0,0.54);
  border-left: none;
}
`

const FormItem = styled(Box)`
  margin-bottom: ${props => props.theme.space[4]}px;
  max-width: 500px;
`
const OptionalFieldText = ({ }) => (
  <Text style={{ display: 'inline', lineHeight: '12px' }} fontWeight="lighter" fontSize="1">{' '}(optional)</Text>
)

// /**
//  * parseGitHubUrl parses a URL for a github repository. It throws if parsing the URL fails
//  * or the URL doesn't look like a github repository URL
//  * @param {string} repoUrl - repository URL with protocol
//  * @returns {repository: string, repositoryOwner: tring} - object containing repository and repository owner
//  */
// export function parseGitHubUrl(repoUrl: string): { repository: string, repositoryOwner: string } {
//   const u = new URL(repoUrl)
//   const paths = u.pathname.split('/')
//   // expected length is 3, since pathname starts with a /, so paths[0] should be empty
//   if (paths.length < 3) {
//     throw new Error("URL expected to be in the format <host>/<owner>/<repository>")
//   }
//   return {
//     repositoryOwner: paths[1],
//     repository: paths[2],
//   }
// }

const requireValidRepository =
  value =>
    () => {
      console.log("value:", value)
      if (!value) {
        return { valid: false, message: 'Repository is required' }
      }
      let repoAddr = value.trim();
      if (!repoAddr) {
        return { valid: false, message: 'Repository is required' }
      }

      console.log("repoAddr", repoAddr)

      // add protocol if user omited it
      if (!repoAddr.startsWith('http://') && !repoAddr.startsWith('https://')) {
        repoAddr = `https://${repoAddr}`
      }

      // TODO: some enterprise github host may have slugs?

      let url
      console.log("repoAddr", repoAddr)
      try {
        url = new URL(repoAddr)
      } catch (e) {
        return { valid: false, message: 'Must be a valid URL' }
      }

      const paths = url.pathname.split('/')
      // expected length is 3, since pathname starts with a /, so paths[0] should be empty
      if (paths.length < 3) {
        return { valid: false, message: 'URL expected to be in the format https://<host>/<owner>/<repository>' }
      }

      // TODO: is this true if enterprise github host?
      console.log(paths)

      const owner = paths[1]
      const repo = paths[2]
      if (owner.trim() === "" || repo.trim() == "") {
        return { valid: false, message: 'URL expected to be in the format https://<host>/<owner>/<repository>' }
      }

      return { valid: true }
    };
