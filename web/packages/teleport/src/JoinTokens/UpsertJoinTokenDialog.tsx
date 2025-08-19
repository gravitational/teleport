/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { useEffect, useMemo, useState } from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonIcon,
  ButtonPrimary,
  ButtonSecondary,
  ButtonText,
  Flex,
  Indicator,
  Text,
} from 'design';
import { Cross } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import { useAsync } from 'shared/hooks/useAsync';

import { useTeleport } from 'teleport';
import {
  AWSRules,
  CreateJoinTokenRequest,
  JoinMethod,
  JoinRole,
  JoinToken,
} from 'teleport/services/joinToken';

import 'teleport/services/resources';

import { Info } from 'design/Alert/Alert';
import { collectKeys } from 'shared/utils/collectKeys';

import auth from 'teleport/services/auth';
import { YamlSupportedResourceKind } from 'teleport/services/yaml/types';

import {
  JoinTokenGCPForm,
  JoinTokenIAMForm,
  JoinTokenOracleForm,
} from './JoinTokenForms';
import { JoinTokenGithubForm } from './JoinTokenGithubForm';

const maxWidth = '550px';

const joinRoleOptions: OptionJoinRole[] = [
  'App',
  'Node',
  'Db',
  'Kube',
  'Bot',
  'WindowsDesktop',
  'Discovery',
].map(role => ({ value: role as JoinRole, label: role as JoinRole }));

const availableJoinMethods: OptionJoinMethod[] = [
  'iam',
  'gcp',
  'oracle',
  'github',
].map(method => ({
  value: method as JoinMethod,
  label: method as JoinMethod,
}));

export type AllowOption = Option<string, string>;
type OptionJoinMethod = Option<JoinMethod, JoinMethod>;
type OptionJoinRole = Option<JoinRole, JoinRole>;
type NewJoinTokenGCPState = {
  project_ids: AllowOption[];
  service_accounts: AllowOption[];
  locations: AllowOption[];
};
type NewJoinTokenOracleState = {
  tenancy: string;
  parent_compartments: AllowOption[];
  regions: AllowOption[];
};

type NewJoinTokenGithubState = {
  server_host?: string;
  static_jwks?: string | undefined;
  enterprise_slug?: string | undefined;
  rules: {
    repository?: string;
    repository_owner?: string | undefined;
    workflow?: string | undefined;
    environment?: string | undefined;
    ref?: string;
    ref_type?: 'any' | 'branch' | 'tag';
  }[];
};

export type NewJoinTokenState = {
  name: string;
  // bot_name is only required when Bot is selected in the roles
  bot_name?: string;
  method: OptionJoinMethod;
  roles: OptionJoinRole[];
  iam: AWSRules[];
  gcp: NewJoinTokenGCPState[];
  oracle: NewJoinTokenOracleState[];
  github: NewJoinTokenGithubState;
};

export const defaultNewTokenState: NewJoinTokenState = {
  name: '',
  bot_name: '',
  method: { value: 'iam', label: 'iam' },
  roles: [],
  iam: [{ aws_account: '', aws_arn: '' }],
  gcp: [{ project_ids: [], service_accounts: [], locations: [] }],
  oracle: [{ tenancy: '', parent_compartments: [], regions: [] }],
  github: {
    rules: [
      {
        ref_type: 'any',
      },
    ],
  },
};

function makeDefaultEditState(token: JoinToken): NewJoinTokenState {
  return {
    name: token.id,
    bot_name: token.bot_name,
    method: {
      value: token.method,
      label: token.method,
    } as OptionJoinMethod,
    roles: token.roles.map(r => ({ value: r, label: r })) as OptionJoinRole[],
    iam: token.allow,
    gcp: token.gcp?.allow.map(r => ({
      project_ids: r.project_ids?.map(i => ({ value: i, label: i })),
      service_accounts: r.service_accounts?.map(i => ({ value: i, label: i })),
      locations: r.locations?.map(i => ({ value: i, label: i })),
    })),
    oracle: token.oracle?.allow.map(r => ({
      tenancy: r.tenancy,
      parent_compartments: r.parent_compartments?.map(i => ({
        value: i,
        label: i,
      })),
      regions: r.regions?.map(i => ({ value: i, label: i })),
    })),
    github: token.github
      ? {
          server_host: token.github.enterprise_server_host,
          enterprise_slug: token.github.enterprise_slug,
          static_jwks: token.github.static_jwks,
          rules: token.github.allow.map(r => ({
            repository: r.repository,
            actor: r.actor,
            environment: r.environment,
            ref: r.ref,
            ref_type: parseGithubRefType(r.ref_type),
            repository_owner: r.repository_owner,
            workflow: r.workflow,
          })),
        }
      : undefined,
  };
}

function parseGithubRefType(refType: string) {
  if (refType == 'branch') {
    return 'branch';
  } else if (refType == 'tag') {
    return 'tag';
  } else {
    return 'any';
  }
}

export const UpsertJoinTokenDialog = ({
  onClose,
  updateTokenList,
  editToken,
  editTokenWithYAML,
}: {
  onClose(): void;
  updateTokenList: (token: JoinToken) => void;
  editToken?: JoinToken;
  editTokenWithYAML: (tokenId: string) => void;
}) => {
  const ctx = useTeleport();
  const [newTokenState, setNewTokenState] = useState<NewJoinTokenState>(
    editToken ? makeDefaultEditState(editToken) : defaultNewTokenState
  );

  const [createTokenAttempt, runCreateTokenAttempt] = useAsync(
    async (req: CreateJoinTokenRequest, isEdit: boolean) => {
      // The edit and create endpoint each call other endpoints
      // that require re-authenticating. Providing a reusable mfaResponse
      // is required for multple internal validations to succeed.
      const mfaResponse = await auth.getMfaChallengeResponseForAdminAction(
        true /* allow re-use */
      );
      const token = isEdit
        ? await ctx.joinTokenService.editJoinToken(req, mfaResponse)
        : await ctx.joinTokenService.createJoinToken(req, mfaResponse);
      updateTokenList(token);
      onClose();
    }
  );

  const [parseYAMLAttempt, parseYAML] = useAsync(
    async (yaml: string): Promise<unknown | null> => {
      return {
        yaml,
        object: await ctx.yamlService.parse<unknown>(
          YamlSupportedResourceKind.ProvisionToken,
          {
            yaml,
          }
        ),
      };
    }
  );

  // Convert the resource YAML to an object for compatibility checking
  useEffect(() => {
    if (editToken) {
      parseYAML(editToken.content);
    }
    // parseYAML is not stable
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editToken]);

  const hasUnsupportedFields = useMemo(() => {
    if (!editToken) {
      return false;
    }

    const { data } = parseYAMLAttempt;

    if (!checkYAMLData(data)) {
      return true;
    }

    switch (data.object.spec.join_method) {
      case 'iam':
        return !checkYamlData(
          data.object.spec.allow,
          data.object.spec.join_method
        );
      case 'gcp':
        return !checkYamlData(
          data.object.spec.gcp,
          data.object.spec.join_method
        );
      case 'github':
        return !checkYamlData(
          data.object.spec.github,
          data.object.spec.join_method
        );
    }

    return false;
  }, [editToken, parseYAMLAttempt]);

  function reset(validator: Validator) {
    validator.reset();
    setNewTokenState(defaultNewTokenState);
  }

  async function save(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    const request: CreateJoinTokenRequest = {
      name: newTokenState.name,
      roles: newTokenState.roles.map(r => r.value),
      join_method: newTokenState.method.value,
    };

    if (newTokenState.method.value === 'iam') {
      request.allow = newTokenState.iam;
    }

    if (request.roles.includes('Bot')) {
      request.bot_name = newTokenState.bot_name;
    }

    if (newTokenState.method.value === 'gcp') {
      const gcp = {
        allow: newTokenState.gcp.map(rule => ({
          project_ids: rule.project_ids?.map(id => id.value),
          locations: rule.locations?.map(loc => loc.value),
          service_accounts: rule.service_accounts?.map(
            account => account.value
          ),
        })),
      };
      request.gcp = gcp;
    }

    if (newTokenState.method.value === 'oracle') {
      const oracle = {
        allow: newTokenState.oracle.map(rule => ({
          tenancy: rule.tenancy,
          parent_compartments: rule.parent_compartments?.map(
            compartment => compartment.value
          ),
          regions: rule.regions?.map(region => region.value),
        })),
      };
      request.oracle = oracle;
    }

    if (newTokenState.method.value === 'github') {
      const github: (typeof request)['github'] = {
        allow: newTokenState.github.rules.map(rule => ({
          environment: rule.environment,
          ref: rule.ref,
          ref_type: rule.ref ? rule.ref_type : undefined,
          repository: rule.repository,
          repository_owner: rule.repository_owner,
          workflow: rule.workflow,

          actor: null, // Unsupported field
          sub: null, // Unsupported field
        })),
        enterprise_server_host: newTokenState.github.server_host,
        enterprise_slug: newTokenState.github.enterprise_slug,
        static_jwks: newTokenState.github.static_jwks,
      };

      request.github = github;
    }

    runCreateTokenAttempt(request, !!editToken);
  }

  function setTokenRoles(roles: OptionJoinRole[]) {
    setNewTokenState(prevState => ({
      ...prevState,
      roles: roles || [],
    }));
  }

  function setTokenMethod(method: OptionJoinMethod) {
    // set the method and reset the token rules per type for a fresh form
    setNewTokenState(prevState => ({
      ...prevState,
      method,
      iam: [{ aws_account: '', aws_arn: '' }], // default
    }));
  }

  function setTokenField(fieldName: string, value: string) {
    setNewTokenState(prevState => ({
      ...prevState,
      [fieldName]: value,
    }));
  }

  return (
    <>
      {parseYAMLAttempt.status === 'processing' && (
        <Flex justifyContent="center">
          <Indicator />
        </Flex>
      )}

      {parseYAMLAttempt.status === 'error' && (
        <Alert kind="danger">{parseYAMLAttempt.statusText}</Alert>
      )}

      {parseYAMLAttempt.status !== 'error' ? (
        <Flex width="500px">
          <Box
            css={`
              overflow: auto;
              width: 100%;
              min-height: 50vh;
              padding-bottom: 0;
            `}
          >
            <Flex
              alignItems="center"
              mb={3}
              justifyContent="space-between"
              maxWidth={maxWidth}
            >
              <Flex alignItems="center" mr={3}>
                <HoverTooltip tipContent="Back to Join Tokens">
                  <ButtonIcon onClick={onClose} mr={2}>
                    <Cross size="medium" />
                  </ButtonIcon>
                </HoverTooltip>
                <Text typography="h3" fontWeight={400}>
                  {editToken ? `Edit Token` : 'Create a New Join Token'}
                </Text>
              </Flex>

              {editToken && !hasUnsupportedFields && (
                <ButtonText
                  p={2}
                  onClick={() => {
                    onClose();
                    editTokenWithYAML(editToken.id);
                  }}
                >
                  <Text color="buttons.link.default">Use YAML editor</Text>
                </ButtonText>
              )}
            </Flex>

            {hasUnsupportedFields ? (
              <Info alignItems="flex-start">
                <Text>
                  This token has configuration that is not visible. To edit this
                  token please use the YAML editor.
                </Text>
                <ButtonSecondary
                  size="large"
                  my={2}
                  onClick={() => {
                    onClose();
                    editTokenWithYAML(editToken.id);
                  }}
                >
                  Use YAML editor
                </ButtonSecondary>
              </Info>
            ) : undefined}

            <Validation>
              {({ validator }) => (
                <Box maxWidth={maxWidth}>
                  {createTokenAttempt.status === 'error' && (
                    <Alert kind="danger">{createTokenAttempt.statusText}</Alert>
                  )}
                  {!editToken && ( // We only want to change the method when creating a new token
                    <FieldSelect
                      label="Method"
                      rule={requiredField<Option>('Select a join method')}
                      isSearchable
                      isClearable={false}
                      value={newTokenState.method}
                      onChange={setTokenMethod}
                      options={availableJoinMethods}
                    />
                  )}
                  {newTokenState.method.value !== 'token' && ( // if the method is token, we generate the name for them on the backend
                    <FieldInput
                      label="Token name"
                      data-testid="name_field"
                      toolTipContent={
                        editToken ? 'Editing token names is not supported.' : ''
                      }
                      rule={requiredField('Token name is required')}
                      placeholder={`${newTokenState.method.value}-token-name`}
                      autoFocus
                      value={newTokenState.name}
                      onChange={e => setTokenField('name', e.target.value)}
                      readonly={!!editToken}
                    />
                  )}
                  <FieldSelect
                    label="Join Roles"
                    inputId="role_select"
                    rule={requiredField('At least one role is required')}
                    placeholder="Click to select roles"
                    isSearchable
                    isMulti
                    mb={5}
                    isClearable={false}
                    value={newTokenState.roles}
                    onChange={setTokenRoles}
                    options={joinRoleOptions}
                    isDisabled={hasUnsupportedFields}
                  />
                  {newTokenState.roles.some(i => i.value === 'Bot') && ( // if Bot is included, we must get a bot name as well
                    <FieldInput
                      label="Bot name"
                      toolTipContent="Bot names are required when the Bot role is selected"
                      rule={requiredField('Bot name is required')}
                      placeholder="Enter bot name"
                      value={newTokenState.bot_name}
                      onChange={e => setTokenField('bot_name', e.target.value)}
                      readonly={hasUnsupportedFields}
                    />
                  )}
                  {newTokenState.method.value === 'iam' && (
                    <JoinTokenIAMForm
                      tokenState={newTokenState}
                      onUpdateState={newState => setNewTokenState(newState)}
                      readonly={hasUnsupportedFields}
                    />
                  )}
                  {newTokenState.method.value === 'gcp' && (
                    <JoinTokenGCPForm
                      tokenState={newTokenState}
                      onUpdateState={newState => setNewTokenState(newState)}
                      readonly={hasUnsupportedFields}
                    />
                  )}
                  {newTokenState.method.value === 'oracle' && (
                    <JoinTokenOracleForm
                      tokenState={newTokenState}
                      onUpdateState={newState => setNewTokenState(newState)}
                      readonly={hasUnsupportedFields}
                    />
                  )}
                  {newTokenState.method.value === 'github' && (
                    <JoinTokenGithubForm
                      tokenState={newTokenState}
                      onUpdateState={setNewTokenState}
                      readonly={hasUnsupportedFields}
                    />
                  )}
                  <Flex
                    mt={4}
                    py={4}
                    gap={2}
                    css={`
                      position: sticky;
                      bottom: 0;
                      background: ${({ theme }) => theme.colors.levels.sunken};
                      border-top: 1px solid
                        ${({ theme }) => theme.colors.spotBackground[1]};
                    `}
                  >
                    <ButtonPrimary
                      width="100%"
                      size="large"
                      textTransform="none"
                      onClick={() => save(validator)}
                      disabled={
                        createTokenAttempt.status === 'processing' ||
                        hasUnsupportedFields
                      }
                    >
                      {editToken ? 'Edit' : 'Create'} Join Token
                    </ButtonPrimary>
                    <ButtonSecondary
                      width="100%"
                      textTransform="none"
                      size="large"
                      onClick={() => {
                        reset(validator);
                        onClose();
                      }}
                      disabled={false}
                    >
                      Cancel
                    </ButtonSecondary>
                  </Flex>
                </Box>
              )}
            </Validation>
          </Box>
        </Flex>
      ) : undefined}
    </>
  );
};

const checkYAMLData = (
  data: unknown
): data is {
  object: {
    spec: {
      join_method: string;
      allow: unknown;
      gcp: unknown;
      github: unknown;
    };
  };
} => {
  if (
    !data ||
    typeof data !== 'object' ||
    data === null ||
    !('object' in data)
  ) {
    return false;
  }

  const { object } = data;

  if (
    !object ||
    typeof object !== 'object' ||
    object === null ||
    !('spec' in object)
  ) {
    return false;
  }

  const { spec } = object;

  if (
    !spec ||
    typeof spec !== 'object' ||
    spec === null ||
    !('join_method' in spec)
  ) {
    return false;
  }

  return typeof spec.join_method === 'string';
};

export const RuleBox = styled(Box)`
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  border-width: 2px;
  border-style: solid;
  border-radius: ${props => props.theme.radii[2]}px;

  margin-bottom: ${props => props.theme.space[3]}px;

  padding: ${props => props.theme.space[3]}px;
`;

const checkYamlData = (data: unknown, joinMethod: JoinMethod) => {
  const supportedFields = supportedFieldsMap[joinMethod];
  if (!supportedFields) {
    return true;
  }
  const keys = collectKeys(data);
  return !keys || new Set(keys).isSubsetOf(supportedFields);
};

const supportedFieldsMap: Partial<Record<JoinMethod, Set<string>>> = {
  iam: new Set(['.aws_account', '.aws_arn']),
  gcp: new Set([
    '.allow.project_ids',
    '.allow.locations',
    '.allow.service_accounts',
  ]),
  github: new Set([
    '.enterprise_server_host',
    '.static_jwks',
    '.enterprise_slug',
    '.allow.repository',
    '.allow.repository_owner',
    '.allow.workflow',
    '.allow.environment',
    '.allow.ref',
    '.allow.ref_type',
  ]),
};
