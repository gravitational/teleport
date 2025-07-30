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

import { useState, type JSX } from 'react';
import styled from 'styled-components';

import { H2, Text } from 'design';
import { Alert } from 'design/Alert';
import Box from 'design/Box';
import { ButtonSecondary } from 'design/Button';
import ButtonIcon from 'design/ButtonIcon';
import Card from 'design/Card';
import Flex from 'design/Flex';
import * as Icons from 'design/Icon';
import Input from 'design/Input';
import Link from 'design/Link';
import FieldInput from 'shared/components/FieldInput';
import Select from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';

import cfg from 'teleport/config';

import { FlowButtons } from '../Shared/FlowButtons';
import { FlowStepProps } from '../Shared/GuidedFlow';
import {
  GITHUB_HOST,
  parseRepoAddress,
  RefTypeOption,
  Rule,
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

  const [hostError, setHostError] = useState<JSX.Element | null>(null);

  function handleNext(validator: Validator) {
    // clear errors
    setHostError(null);

    if (!validator.validate()) {
      return;
    }

    const hosts = new Set<string>();
    repoRules.forEach(rule => {
      try {
        const { host } = parseRepoAddress(rule.repoAddress);
        hosts.add(host);
      } catch (err) {
        setHostError(InvalidHostError({ rule: rule.repoAddress, error: err }));
      }
    });

    // ensure all repositories have the same host
    if (hosts.size > 1) {
      setHostError(MultipleHostsError);
      return;
    }

    const isGitHubEnterpriseHost = [...hosts][0] !== GITHUB_HOST;
    // ensure only enterprise users can use GitHub Enterprise Server Host
    if (isGitHubEnterpriseHost && !cfg.isEnterprise) {
      setHostError(EnterpriseHostError);
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
            <H2 mb="3">Step 2: Input Your GitHub Account Info</H2>
            <Text mb="3">
              These fields will be combined with your bot's permissions to
              create a join token and generate a sample GitHub Actions file.
            </Text>

            {repoRules.map((rule, i) => (
              <Card p="4" maxWidth="540px" key={i} mb="4">
                <Box>
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
                      placeholder="https://github.com/gravitational/teleport"
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
                          placeholder="main"
                          style={{ borderRadius: '4px 0 0 4px' }}
                          value={repoRules[i].ref}
                          onChange={e => handleChange(i, 'ref', e.target.value)}
                        />
                      </Box>
                      <Box minWidth="160px">
                        <Text ml="1">Ref Type</Text>
                        <RefTypeSelect
                          isDisabled={isLoading}
                          isMulti={false}
                          value={repoRules[i].refType}
                          onChange={o => handleChange(i, 'refType', o)}
                          options={refTypeOptions}
                          menuPlacement="auto"
                        />
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
                      placeholder="cd"
                      value={repoRules[i].workflowName}
                      onChange={e =>
                        handleChange(i, 'workflowName', e.target.value)
                      }
                    />
                  </FormItem>

                  <FormItem>
                    <Text>
                      Environment <OptionalFieldText />
                    </Text>
                    <Input
                      disabled={isLoading}
                      placeholder="development"
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
                      placeholder="octocat"
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
                </Box>
              </Card>
            ))}
            <Box mb="4">
              {attempt.status === 'failed' && (
                <Alert kind="danger">{attempt.statusText}</Alert>
              )}
              {hostError && <Alert kind="danger">{hostError}</Alert>}
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

const RefTypeSelect = styled(Select<RefTypeOption>)`
  .react-select__control {
    border-radius: 0 4px 4px 0;
    border-left-color: transparent;
  }

  .react-select__control--is-focused {
    border-left-color: ${props =>
      props.theme.colors.interactive.solid.primary.default};
  }
`;

const FormItem = styled(Box)`
  margin-bottom: ${props => props.theme.space[4]}px;
  max-width: 500px;
`;
const OptionalFieldText = () => (
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

const MultipleHostsError = () => {
  return (
    <Text>
      All repositories must be in the same host. Please create different bots
      for each host.
    </Text>
  );
};

const EnterpriseHostError = () => {
  return (
    <Box>
      GitHub Enterprise Server Host require Teleport Enterprise.Please use a
      repository hosted at github.com or{' '}
      <Link target="_blank" href="https://goteleport.com/signup/enterprise/">
        contact us
      </Link>
      .
    </Box>
  );
};
const InvalidHostError = ({ rule, error }: { rule: string; error: string }) => {
  return (
    <Box>
      Invalid address {rule}: {error}
    </Box>
  );
};
