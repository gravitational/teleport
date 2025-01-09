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

import { useState } from 'react';
import styled from 'styled-components';

import {
  Alert,
  Box,
  ButtonIcon,
  ButtonPrimary,
  ButtonSecondary,
  ButtonText,
  Flex,
  Text,
} from 'design';
import { Cross } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import Validation from 'shared/components/Validation';
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

import { JoinTokenGCPForm, JoinTokenIAMForm } from './JoinTokenForms';

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

const availableJoinMethods: OptionJoinMethod[] = ['iam', 'gcp'].map(method => ({
  value: method as JoinMethod,
  label: method as JoinMethod,
}));

export type OptionGCP = Option<string, string>;
type OptionJoinMethod = Option<JoinMethod, JoinMethod>;
type OptionJoinRole = Option<JoinRole, JoinRole>;
type NewJoinTokenGCPState = {
  project_ids: OptionGCP[];
  service_accounts: OptionGCP[];
  locations: OptionGCP[];
};

export type NewJoinTokenState = {
  name: string;
  // bot_name is only required when Bot is selected in the roles
  bot_name?: string;
  method: OptionJoinMethod;
  roles: OptionJoinRole[];
  iam: AWSRules[];
  gcp: NewJoinTokenGCPState[];
};

export const defaultNewTokenState: NewJoinTokenState = {
  name: '',
  bot_name: '',
  method: { value: 'iam', label: 'iam' },
  roles: [],
  iam: [{ aws_account: '', aws_arn: '' }],
  gcp: [{ project_ids: [], service_accounts: [], locations: [] }],
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
  };
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
    async (req: CreateJoinTokenRequest) => {
      const token = await ctx.joinTokenService.createJoinToken(req);
      updateTokenList(token);
      onClose();
    }
  );

  function reset(validator) {
    validator.reset();
    setNewTokenState(defaultNewTokenState);
  }

  async function save(validator) {
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

    runCreateTokenAttempt(request);
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
    <Flex width="500px">
      <Box
        pr={4}
        pl={4}
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
              <ButtonIcon onClick={onClose} mr={2} ml={'-8px'}>
                <Cross size="medium" />
              </ButtonIcon>
            </HoverTooltip>
            <Text typography="h3" fontWeight={400}>
              {editToken ? `Edit Token` : 'Create a New Join Token'}
            </Text>
          </Flex>
          {editToken && (
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
                  placeholder={
                    newTokenState.method.value === 'iam'
                      ? 'iam-token-name'
                      : 'gcp-token-name'
                  }
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
              />
              {newTokenState.roles.some(i => i.value === 'Bot') && ( // if Bot is included, we must get a bot name as well
                <FieldInput
                  label="Bot name"
                  toolTipContent="Bot names are required when the Bot role is selected"
                  rule={requiredField('Bot name is required')}
                  placeholder="Enter bot name"
                  value={newTokenState.bot_name}
                  onChange={e => setTokenField('bot_name', e.target.value)}
                />
              )}
              {newTokenState.method.value === 'iam' && (
                <JoinTokenIAMForm
                  tokenState={newTokenState}
                  onUpdateState={newState => setNewTokenState(newState)}
                />
              )}
              {newTokenState.method.value === 'gcp' && (
                <JoinTokenGCPForm
                  tokenState={newTokenState}
                  onUpdateState={newState => setNewTokenState(newState)}
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
                    ${props => props.theme.colors.spotBackground[1]};
                `}
              >
                <ButtonPrimary
                  width="100%"
                  size="large"
                  textTransform="none"
                  onClick={() => save(validator)}
                  disabled={createTokenAttempt.status === 'processing'}
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
  );
};

export const RuleBox = styled(Box)`
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  border-width: 2px;
  border-style: solid;
  border-radius: ${props => props.theme.radii[2]}px;

  margin-bottom: ${props => props.theme.space[3]}px;

  padding: ${props => props.theme.space[3]}px;
`;
