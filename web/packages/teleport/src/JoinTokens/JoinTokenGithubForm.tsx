/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import styled from 'styled-components';

import { Info } from 'design/Alert/Alert';
import Box from 'design/Box/Box';
import { ButtonText } from 'design/Button/Button';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import Flex from 'design/Flex/Flex';
import { Plus } from 'design/Icon/Icons/Plus';
import { Trash } from 'design/Icon/Icons/Trash';
import Link from 'design/Link/Link';
import Text from 'design/Text/Text';
import FieldInput from 'shared/components/FieldInput/FieldInput';
import FieldSelect from 'shared/components/FieldSelect';
import { requiredField } from 'shared/components/Validation/rules';

import cfg from 'teleport/config';
import { SectionBox } from 'teleport/Roles/RoleEditor/StandardEditor/sections';

import { NewJoinTokenState } from './UpsertJoinTokenDialog';

export const JoinTokenGithubForm = ({
  tokenState,
  onUpdateState,
  readonly,
}: {
  tokenState: NewJoinTokenState;
  onUpdateState: (newToken: NewJoinTokenState) => void;
  readonly: boolean;
}) => {
  const { github } = tokenState;
  const { rules } = github;

  function removeRule(index: number) {
    const newRules = rules.filter((_, i) => index !== i);
    const newState = {
      ...tokenState,
      github: {
        ...tokenState.github,
        rules: newRules,
      },
    };
    onUpdateState(newState);
  }

  function addNewRule() {
    const newState = {
      ...tokenState,
      github: {
        ...tokenState.github,
        rules: [
          ...rules,
          {
            ref_type: 'any' as const,
          },
        ],
      },
    };
    onUpdateState(newState);
  }

  function updateRuleField(index: number, fieldName: string, opts: unknown) {
    const newState = {
      ...tokenState,
      github: {
        ...tokenState.github,
        rules: rules.map((rule, i) => {
          if (i === index) {
            return { ...rule, [fieldName]: opts };
          }
          return rule;
        }),
      },
    };
    onUpdateState(newState);
  }

  return (
    <>
      {rules.map((rule, index) => (
        <RuleBox
          data-testid={`rule_${index}`}
          key={index} // order doesn't change without updating the referrenced array
        >
          <Flex alignItems="center" justifyContent="space-between">
            <Text fontWeight={700} mb={2}>
              GitHub Rule
            </Text>

            {rules.length > 1 && ( // at least one rule is required, so lets not allow the user to remove it
              <ButtonIcon
                data-testid="delete_rule"
                onClick={() => removeRule(index)}
              >
                <Trash size={16} color="text.muted" />
              </ButtonIcon>
            )}
          </Flex>

          <FieldInput
            label="Repository"
            placeholder="gravitational/teleport"
            toolTipContent="Fully qualified name of a GitHub repository (i.e. including the owner)"
            value={rule.repository}
            onChange={e => updateRuleField(index, 'repository', e.target.value)}
            rule={
              rule.repository_owner
                ? undefined
                : requiredField('Either repository name or owner is required')
            }
            readonly={readonly}
          />

          <FieldInput
            label="Repository owner"
            placeholder="gravitational"
            toolTipContent="Name of an organization or user that a repository belongs to"
            value={rule.repository_owner}
            onChange={e =>
              updateRuleField(index, 'repository_owner', e.target.value)
            }
            rule={
              rule.repository
                ? undefined
                : requiredField('Either repository owner or name is required')
            }
            readonly={readonly}
          />

          <FieldInput
            label="Workflow name"
            placeholder="my-workflow"
            value={rule.workflow}
            onChange={e => updateRuleField(index, 'workflow', e.target.value)}
            readonly={readonly}
          />

          <FieldInput
            label="Environment"
            placeholder="production"
            value={rule.environment}
            onChange={e =>
              updateRuleField(index, 'environment', e.target.value)
            }
            readonly={readonly}
          />

          <Flex fullWidth gap={2}>
            <FieldInput
              flex={2}
              label="Git ref"
              placeholder="ref/heads/main"
              value={rule.ref}
              onChange={e => updateRuleField(index, 'ref', e.target.value)}
              readonly={readonly}
            />

            <FieldSelect
              flex={1}
              label="Ref type"
              options={refTypeOptions}
              value={refTypeOptions.find(o => o.value === rule.ref_type)}
              onChange={opts => updateRuleField(index, 'ref_type', opts.value)}
              isDisabled={!rule.ref || readonly}
              data-testid="ref-type-select"
            />
          </Flex>
        </RuleBox>
      ))}

      <ButtonText onClick={addNewRule} compact mb={4}>
        <Plus size={16} mr={2} />
        Add another GitHub rule
      </ButtonText>

      {/* TODO(nickmarais): Make SectionBox a component instead of reusing it from Roles */}
      <SectionBox
        titleSegments={['GitHub Enterprise Server']}
        initiallyCollapsed={[
          'server_host',
          'enterprise_slug',
          'static_jwks',
        ].every(k => !github[k])}
        validation={{
          valid: true,
        }}
      >
        <Text fontWeight="regular" mb={3}>
          Additional settings for configuring GHES.
        </Text>

        {cfg.edition !== 'ent' ? (
          <Info alignItems="flex-start">
            GitHub Enterprise Server configuration requires Teleport Enterprise.
            Please use a repository hosted at github.com or{' '}
            <Link
              target="_blank"
              href="https://goteleport.com/signup/enterprise/"
            >
              contact us
            </Link>
            .
          </Info>
        ) : undefined}

        <FieldInput
          label="Server host"
          placeholder="github.example.com"
          value={github.server_host}
          onChange={e =>
            onUpdateState({
              ...tokenState,
              github: {
                ...tokenState.github,
                server_host: e.target.value,
              },
            })
          }
          readonly={readonly || cfg.edition !== 'ent'}
        />

        <FieldInput
          label="Slug"
          placeholder="octo-enterprise"
          value={github.enterprise_slug}
          onChange={e =>
            onUpdateState({
              ...tokenState,
              github: {
                ...tokenState.github,
                enterprise_slug: e.target.value,
              },
            })
          }
          readonly={readonly || cfg.edition !== 'ent'}
        />

        <FieldInput
          label="Static JWKS"
          placeholder='{"keys":[--snip--]}'
          toolTipContent="JSON Web Key Set used to verify the token issued by GitHub Actions"
          value={github.static_jwks}
          onChange={e =>
            onUpdateState({
              ...tokenState,
              github: {
                ...tokenState.github,
                static_jwks: e.target.value,
              },
            })
          }
          readonly={readonly || cfg.edition !== 'ent'}
        />
      </SectionBox>
    </>
  );
};

const refTypeOptions = [
  { value: 'any' as const, label: 'Any' },
  { value: 'branch' as const, label: 'Branch' },
  { value: 'tag' as const, label: 'Tag' },
];

const RuleBox = styled(Box)`
  border-color: ${props => props.theme.colors.interactive.tonal.neutral[0]};
  border-width: 2px;
  border-style: solid;
  border-radius: ${props => props.theme.radii[2]}px;

  margin-bottom: ${props => props.theme.space[3]}px;

  padding: ${props => props.theme.space[3]}px;
`;
