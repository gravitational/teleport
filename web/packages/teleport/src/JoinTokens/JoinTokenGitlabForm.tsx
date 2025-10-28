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

import Box from 'design/Box/Box';
import { ButtonText } from 'design/Button/Button';
import ButtonIcon from 'design/ButtonIcon/ButtonIcon';
import Flex from 'design/Flex/Flex';
import { Plus } from 'design/Icon/Icons/Plus';
import { Trash } from 'design/Icon/Icons/Trash';
import Text from 'design/Text';
import FieldInput from 'shared/components/FieldInput/FieldInput';
import { FieldSelect } from 'shared/components/FieldSelect/FieldSelect';
import { requiredField } from 'shared/components/Validation/rules';

import { SectionBox } from 'teleport/Roles/RoleEditor/StandardEditor/sections';

import { NewJoinTokenState } from './UpsertJoinTokenDialog';

export function JoinTokenGitlabForm(props: {
  tokenState: NewJoinTokenState;
  onUpdateState: (newToken: NewJoinTokenState) => void;
  readonly: boolean;
}) {
  const { tokenState, onUpdateState, readonly } = props;
  const { gitlab } = tokenState;
  const { domain, static_jwks, rules } = gitlab ?? {};

  function removeRule(index: number) {
    const newRules = rules?.filter((_, i) => index !== i);
    const newState = {
      ...tokenState,
      gitlab: {
        ...tokenState.gitlab,
        rules: newRules,
      },
    };
    onUpdateState(newState);
  }

  function addNewRule() {
    const newState = {
      ...tokenState,
      gitlab: {
        ...tokenState.gitlab,
        rules: [
          ...(rules ?? []),
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
      gitlab: {
        ...tokenState.gitlab,
        rules: rules?.map((rule, i) => {
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
      {rules?.map((rule, index) => (
        <RuleBox
          data-testid={`rule_${index}`}
          key={index} // order doesn't change without updating the referrenced array
        >
          <Flex alignItems="center" justifyContent="space-between">
            <Text fontWeight={700} mb={2}>
              GitLab Rule
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
            label="Project path"
            placeholder="my-user/my-project"
            value={rule.project_path ?? ''}
            onChange={e =>
              updateRuleField(index, 'project_path', e.target.value)
            }
            rule={
              rule.namespace_path
                ? undefined
                : requiredField(
                    'Either project path or namespace path is required'
                  )
            }
            readonly={readonly}
          />

          <FieldInput
            label="Namespace path"
            placeholder="my-user"
            value={rule.namespace_path ?? ''}
            onChange={e =>
              updateRuleField(index, 'namespace_path', e.target.value)
            }
            rule={
              rule.project_path
                ? undefined
                : requiredField(
                    'Either namespace path or project path is required'
                  )
            }
            readonly={readonly}
          />

          <FieldInput
            label="Environment"
            placeholder="production"
            value={rule.environment ?? ''}
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
              value={rule.ref ?? ''}
              onChange={e => updateRuleField(index, 'ref', e.target.value)}
              readonly={readonly}
            />

            <FieldSelect
              flex={1}
              label="Ref type"
              options={refTypeOptions}
              value={refTypeOptions.find(o => o.value === rule.ref_type)}
              onChange={opts => updateRuleField(index, 'ref_type', opts?.value)}
              isDisabled={!rule.ref || readonly}
              data-testid="ref-type-select"
            />
          </Flex>
        </RuleBox>
      ))}

      <ButtonText onClick={addNewRule} compact mb={4}>
        <Plus size={16} mr={2} />
        Add another GitLab rule
      </ButtonText>

      {/* TODO(nicholasmarais1158): Make SectionBox a component instead of reusing it from Roles */}
      <SectionBox
        titleSegments={['Advanced Configuration']}
        initiallyCollapsed={['domain', 'static_jwks'].every(k => !gitlab?.[k])}
        validation={{
          valid: true,
        }}
      >
        <FieldInput
          label="Domain"
          placeholder="gitlab.example.com"
          toolTipContent="If you are using GitLab's cloud hosted offering, leave this field empty"
          value={domain ?? ''}
          onChange={e =>
            onUpdateState({
              ...tokenState,
              gitlab: {
                ...tokenState.gitlab,
                domain: e.target.value,
              },
            })
          }
          readonly={readonly}
        />

        <FieldInput
          label="Static JWKS"
          placeholder='{"keys":[--snip--]}'
          toolTipContent="JSON Web Key Set used to verify the token issued by GitLab"
          value={static_jwks ?? ''}
          onChange={e =>
            onUpdateState({
              ...tokenState,
              gitlab: {
                ...tokenState.gitlab,
                static_jwks: e.target.value,
              },
            })
          }
          readonly={readonly}
        />
      </SectionBox>
    </>
  );
}

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
