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

import { memo } from 'react';
import { components, MultiValueProps } from 'react-select';
import styled, { useTheme } from 'styled-components';

import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { Plus } from 'design/Icon';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip';
import FieldInput from 'shared/components/FieldInput';
import {
  FieldSelect,
  FieldSelectCreatable,
} from 'shared/components/FieldSelect';
import { precomputed } from 'shared/components/Validation/rules';

import { SectionBox, SectionPropsWithDispatch } from './sections';
import {
  ResourceKindOption,
  resourceKindOptions,
  resourceKindOptionsMap,
  RuleModel,
  verbOptions,
} from './standardmodel';
import { AccessRuleValidationResult } from './validation';

/**
 * Access rules tab. This component is memoized to optimize performance; make
 * sure that the properties don't change unless necessary.
 */
export const AccessRules = memo(function AccessRules({
  value,
  isProcessing,
  validation,
  dispatch,
}: SectionPropsWithDispatch<RuleModel[], AccessRuleValidationResult[]>) {
  function addRule() {
    dispatch({ type: 'add-access-rule' });
  }
  return (
    <Flex flexDirection="column" gap={3}>
      {value.map((rule, i) => (
        <AccessRule
          key={rule.id}
          isProcessing={isProcessing}
          value={rule}
          validation={validation[i]}
          dispatch={dispatch}
        />
      ))}
      <ButtonSecondary alignSelf="start" onClick={addRule}>
        <Plus size="small" mr={2} />
        Add New
      </ButtonSecondary>
    </Flex>
  );
});

const AccessRule = memo(function AccessRule({
  value,
  isProcessing,
  validation,
  dispatch,
}: SectionPropsWithDispatch<RuleModel, AccessRuleValidationResult>) {
  const { id, resources, verbs, where } = value;
  const theme = useTheme();
  function setRule(rule: RuleModel) {
    dispatch({ type: 'set-access-rule', payload: rule });
  }
  function removeRule() {
    dispatch({ type: 'remove-access-rule', payload: { id } });
  }
  return (
    <SectionBox
      title="Access Rule"
      tooltip="A rule that gives users access to certain kinds of resources"
      removable
      isProcessing={isProcessing}
      validation={validation}
      onRemove={removeRule}
    >
      <ResourceKindSelect
        components={{ MultiValue: ResourceKindMultiValue }}
        isMulti
        label="Resources"
        isDisabled={isProcessing}
        options={resourceKindOptions}
        value={resources}
        onChange={r => setRule({ ...value, resources: r })}
        rule={precomputed(validation.fields.resources)}
      />
      <FieldSelect
        isMulti
        label="Permissions"
        isDisabled={isProcessing}
        options={verbOptions}
        value={verbs}
        onChange={v => setRule({ ...value, verbs: v })}
        rule={precomputed(validation.fields.verbs)}
      />
      <FieldInput
        label="Filter"
        toolTipContent={
          <>
            Optional condition that further limits the list of resources
            affected by this rule, expressed using the{' '}
            <Text
              as="a"
              href="https://goteleport.com/docs/reference/predicate-language/"
              target="_blank"
              color={theme.colors.interactive.solid.accent.default}
            >
              Teleport predicate language
            </Text>
          </>
        }
        tooltipSticky
        disabled={isProcessing}
        value={where}
        onChange={e => setRule({ ...value, where: e.target.value })}
        mb={0}
      />
    </SectionBox>
  );
});

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
