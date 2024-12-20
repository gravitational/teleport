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

import Flex from 'design/Flex';

import { ButtonSecondary } from 'design/Button';
import { Plus } from 'design/Icon';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { precomputed } from 'shared/components/Validation/rules';
import { components, MultiValueProps } from 'react-select';
import { HoverTooltip } from 'design/Tooltip';
import styled from 'styled-components';

import { AccessRuleValidationResult } from './validation';
import {
  newRuleModel,
  ResourceKindOption,
  resourceKindOptions,
  resourceKindOptionsMap,
  RuleModel,
  verbOptions,
} from './standardmodel';
import { SectionBox, SectionProps } from './sections';

export function AccessRules({
  value,
  isProcessing,
  validation,
  onChange,
}: SectionProps<RuleModel[], AccessRuleValidationResult[]>) {
  function addRule() {
    onChange?.([...value, newRuleModel()]);
  }
  function setRule(rule: RuleModel) {
    onChange?.(value.map(r => (r.id === rule.id ? rule : r)));
  }
  function removeRule(id: string) {
    onChange?.(value.filter(r => r.id !== id));
  }
  return (
    <Flex flexDirection="column" gap={3}>
      {value.map((rule, i) => (
        <AccessRule
          key={rule.id}
          isProcessing={isProcessing}
          value={rule}
          onChange={setRule}
          validation={validation[i]}
          onRemove={() => removeRule(rule.id)}
        />
      ))}
      <ButtonSecondary alignSelf="start" onClick={addRule}>
        <Plus size="small" mr={2} />
        Add New
      </ButtonSecondary>
    </Flex>
  );
}

function AccessRule({
  value,
  isProcessing,
  validation,
  onChange,
  onRemove,
}: SectionProps<RuleModel, AccessRuleValidationResult> & {
  onRemove?(): void;
}) {
  const { resources, verbs } = value;
  return (
    <SectionBox
      title="Access Rule"
      tooltip="A rule that gives users access to certain kinds of resources"
      removable
      isProcessing={isProcessing}
      validation={validation}
      onRemove={onRemove}
    >
      <ResourceKindSelect
        components={{ MultiValue: ResourceKindMultiValue }}
        isMulti
        label="Resources"
        isDisabled={isProcessing}
        options={resourceKindOptions}
        value={resources}
        onChange={r => onChange?.({ ...value, resources: r })}
        rule={precomputed(validation.fields.resources)}
      />
      <FieldSelect
        isMulti
        label="Permissions"
        isDisabled={isProcessing}
        options={verbOptions}
        value={verbs}
        onChange={v => onChange?.({ ...value, verbs: v })}
        rule={precomputed(validation.fields.verbs)}
        mb={0}
      />
    </SectionBox>
  );
}

const ResourceKindSelect = styled(
  FieldSelectCreatable<ResourceKindOption, true>
)`
  .teleport-resourcekind__value--unknown {
    background: ${props => props.theme.colors.interactive.solid.alert.default};
    .react-select__multi-value__label,
    .react-select__multi-value__remove {
      color: ${props => props.theme.colors.text.primaryInverse};
    }
  }
`;

function ResourceKindMultiValue(props: MultiValueProps<ResourceKindOption>) {
  if (resourceKindOptionsMap.has(props.data.value)) {
    return <components.MultiValue {...props} />;
  }
  return (
    <HoverTooltip tipContent="Unrecognized resource type">
      <components.MultiValue
        {...props}
        className="teleport-resourcekind__value--unknown"
      />
    </HoverTooltip>
  );
}
