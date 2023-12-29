import React, { useState } from 'react';
import styled from 'styled-components';
import ReactSelect from 'react-select';

import Box from 'design/Box';
import Text from 'design/Text';
import * as Icons from 'design/Icon';
import { StyledSelect } from 'shared/components/Select';
import Input from 'design/Input';
import { ButtonSecondary } from 'design/Button';
import Validation, { Validator } from 'shared/components/Validation';

import Flex from 'design/Flex';
import Card from 'design/Card';
import ButtonIcon from 'design/ButtonIcon';
import FieldInput from 'shared/components/FieldInput';

import Alert from 'design/Alert';

import { FlowButtons } from '../shared/FlowButtons';
import { FlowStepProps } from '../shared/GuidedFlow';

import {
  RefTypeOption,
  Rule,
  parseRepoAddress,
  useGitHubFlow,
} from './useGitHubFlow';

const refTypeOptions: RefTypeOption[] = [
  {
    label: 'any',
    value: '',
  },
  {
    label: 'Branch',
    value: 'branch',
  },
  {
    label: 'Tag',
    value: 'tag',
  },
];

export function ConnectGitHub({ nextStep, prevStep }: FlowStepProps) {
  const {
    repoRules,
    setRepoRules,
    addEmptyRepoRule,
    createBot,
    attempt,
    resetAttempt,
  } = useGitHubFlow();
  const isLoading = attempt.status === 'processing';

  const [multipleHostsErr, setMultipleHostsErr] = useState(false);

  function handleNext(validator: Validator) {
    // clear errors
    setMultipleHostsErr(false);

    if (!validator.validate()) {
      return;
    }

    // all repositories should have the same host
    const hosts = new Set<string>();
    repoRules.forEach(rule => {
      const { host } = parseRepoAddress(rule.repoAddress);
      hosts.add(host);
    });

    if (hosts.size > 1) {
      setMultipleHostsErr(true);
      return;
    }

    createBot().then(success => {
      if (success) {
        nextStep();
      }
    });
  }

  function handleChange(
    index: number,
    field: keyof Rule,
    value: Rule[typeof field]
  ) {
    const newRules = [...repoRules];
    newRules[index] = { ...newRules[index], [field]: value };
    setRepoRules(newRules);
  }

  return (
    <Box>
      <Validation>
        {({ validator }) => (
          <Box mt="3">
            <Text bold fontSize={4} mb="3">
              Step 2: Input Your GitHub Account Info
            </Text>
            <Text mb="3">
              These fields will be combined with your machine user’s permissions
              to create a join token and generate a sample GitHub Actions file.
            </Text>

            {repoRules.map((rule, i) => (
              <Card p="4" maxWidth="540px" key={i} mb="4">
                <Box>
                  <>
                    <Flex alignItems="center" justifyContent="space-between">
                      <Text bold>GitHub Repository Access:</Text>
                      {i > 0 && (
                        <ButtonIcon
                          size={1}
                          title="Remove Rule"
                          onClick={() =>
                            setRepoRules(
                              repoRules.filter((r, index) => index !== i)
                            )
                          }
                        >
                          <Icons.Trash size="medium" />
                        </ButtonIcon>
                      )}
                    </Flex>
                    <FormItem>
                      <Text mt="3">Full URL to Your Repository*</Text>
                      <FieldInput
                        disabled={isLoading}
                        rule={requireValidRepository}
                        label=" "
                        placeholder="ex. https://github.com/gravitational/teleport"
                        value={repoRules[i].repoAddress}
                        onChange={e =>
                          handleChange(i, 'repoAddress', e.target.value)
                        }
                      />
                    </FormItem>

                    <FormItem>
                      <Flex>
                        <Box width="100%">
                          <Text>
                            Git Ref <OptionalFieldText />
                          </Text>
                          <Input
                            disabled={isLoading}
                            label="Git Ref"
                            placeholder="main"
                            style={{ borderRadius: '4px 0 0 4px' }}
                            value={repoRules[i].ref}
                            onChange={e =>
                              handleChange(i, 'ref', e.target.value)
                            }
                          />
                        </Box>
                        <Box minWidth="160px">
                          <Text ml="1">Ref Type</Text>
                          <RefTypeSelect>
                            <ReactSelect
                              disabled={isLoading}
                              isMulti={false}
                              value={repoRules[i].refType}
                              onChange={o => handleChange(i, 'refType', o)}
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
                      <Text>
                        Name of the GitHub Actions Workflow
                        <OptionalFieldText />
                      </Text>
                      <FieldInput
                        disabled={isLoading}
                        placeholder="ex. cd"
                        value={repoRules[i].workflowName}
                        onChange={e =>
                          handleChange(i, 'workflowName', e.target.value)
                        }
                      />
                    </FormItem>

                    <FormItem>
                      <Text>
                        Environmnet <OptionalFieldText />
                      </Text>
                      <Input
                        disabled={isLoading}
                        placeholder="ex. development"
                        value={repoRules[i].environment}
                        onChange={e =>
                          handleChange(i, 'environment', e.target.value)
                        }
                      />
                    </FormItem>

                    <Box>
                      <Text>
                        Restrict to a GitHub User
                        <OptionalFieldText />{' '}
                      </Text>
                      <Input
                        disabled={isLoading}
                        placeholder="ex. octocat"
                        value={repoRules[i].actor}
                        onChange={e => handleChange(i, 'actor', e.target.value)}
                      />
                      <Text
                        fontWeight="lighter"
                        fontSize="1"
                        style={{ fontStyle: 'italic' }}
                      >
                        If left blank, any GitHub user can trigger the workflow
                      </Text>
                    </Box>
                  </>
                </Box>
              </Card>
            ))}
            <Box mb="4">
              {attempt.status === 'failed' && (
                <Alert kind="danger">{attempt.statusText}</Alert>
              )}
              {multipleHostsErr && (
                <Alert kind="danger">
                  All repositories must be in the same host. Please create
                  different bots for each host.
                </Alert>
              )}
              <ButtonSecondary disabled={isLoading} onClick={addEmptyRepoRule}>
                + Add Another Set of Repository Rules
              </ButtonSecondary>
            </Box>
            <FlowButtons
              backButton={{
                disabled: isLoading,
                hidden: false,
              }}
              nextButton={{
                disabled: isLoading,
              }}
              nextStep={() => handleNext(validator)}
              prevStep={() => {
                resetAttempt();
                prevStep();
              }}
            />
          </Box>
        )}
      </Validation>
    </Box>
  );
}

const RefTypeSelect = styled(StyledSelect)`
  .react-select__control {
    border-radius: 0 4px 4px 0;
    border-left: none;
  }

  .react-select__control:hover {
    border: 1px solid rgba(0, 0, 0, 0.54);
    border-left: none;
  }
`;

const FormItem = styled(Box)`
  margin-bottom: ${props => props.theme.space[4]}px;
  max-width: 500px;
`;
const OptionalFieldText = ({ }) => (
  <Text
    style={{ display: 'inline', lineHeight: '12px' }}
    fontWeight="lighter"
    fontSize="1"
  >
    {' '}
    (optional)
  </Text>
);

const requireValidRepository = value => () => {
  if (!value) {
    return { valid: false, message: 'Repository is required' };
  }
  let repoAddr = value.trim();
  if (!repoAddr) {
    return { valid: false, message: 'Repository is required' };
  }

  // add protocol if user omited it
  if (!repoAddr.startsWith('http://') && !repoAddr.startsWith('https://')) {
    repoAddr = `https://${repoAddr}`;
  }

  try {
    const { owner, repository } = parseRepoAddress(repoAddr);
    if (owner.trim() === '' || repository.trim() == '') {
      return {
        valid: false,
        message:
          'URL expected to be in the format https://<host>/<owner>/<repository>',
      };
    }

    return { valid: true };
  } catch (e) {
    return { valid: false, message: e?.message };
  }
};
