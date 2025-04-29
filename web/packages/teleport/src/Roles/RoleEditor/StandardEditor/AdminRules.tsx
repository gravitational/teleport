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

import { memo, useId } from 'react';
import { components, MultiValueProps } from 'react-select';
import styled, { useTheme } from 'styled-components';

import { ButtonSecondary } from 'design/Button';
import Flex from 'design/Flex';
import { Plus } from 'design/Icon';
import { LabelContent } from 'design/LabelInput/LabelInput';
import Text from 'design/Text';
import { HoverTooltip } from 'design/Tooltip';
import { FieldCheckbox } from 'shared/components/FieldCheckbox';
import FieldInput from 'shared/components/FieldInput';
import { HelperTextLine } from 'shared/components/FieldInput/FieldInput';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { useRule } from 'shared/components/Validation';
import { precomputed, Rule } from 'shared/components/Validation/rules';
import { ValidationSuspender } from 'shared/components/Validation/Validation';

import { Verb } from 'teleport/services/resources';

import {
  SectionBox,
  SectionPadding,
  SectionPropsWithDispatch,
} from './sections';
import {
  ResourceKindOption,
  resourceKindOptions,
  resourceKindOptionsMap,
  RuleModel,
  VerbModel,
} from './standardmodel';
import { ActionType } from './useStandardModel';
import { AdminRuleValidationResult } from './validation';

/**
 * Admin rules tab. This component is memoized to optimize performance; make
 * sure that the properties don't change unless necessary.
 */
export const AdminRules = memo(function AdminRules({
  value,
  isProcessing,
  validation,
  dispatch,
}: SectionPropsWithDispatch<RuleModel[], AdminRuleValidationResult[]>) {
  function addRule() {
    dispatch({ type: ActionType.AddAdminRule });
  }
  return (
    <Flex flexDirection="column" gap={3}>
      <SectionPadding>
        Rules that give this role administrative rights to Teleport resources
      </SectionPadding>
      {value.map((rule, i) => (
        <AdminRule
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

const AdminRule = memo(function AdminRule({
  value,
  isProcessing,
  validation,
  dispatch,
}: SectionPropsWithDispatch<RuleModel, AdminRuleValidationResult>) {
  const { id, resources, verbs, allVerbs, where, hideValidationErrors } = value;
  const theme = useTheme();
  function setResources(resources: readonly ResourceKindOption[]) {
    dispatch({
      type: ActionType.SetAdminRuleResources,
      payload: { id, resources },
    });
  }
  function setVerbChecked(verb: Verb, checked: boolean) {
    dispatch({
      type: ActionType.SetAdminRuleVerb,
      payload: { id, verb, checked },
    });
  }
  function setAllVerbsChecked(checked: boolean) {
    dispatch({
      type: ActionType.SetAdminRuleAllVerbs,
      payload: { id, checked },
    });
  }
  function setWhere(where: string) {
    dispatch({ type: ActionType.SetAdminRuleWhere, payload: { id, where } });
  }
  function removeRule() {
    dispatch({ type: ActionType.RemoveAdminRule, payload: { id } });
  }
  return (
    <ValidationSuspender suspend={hideValidationErrors}>
      <SectionBox
        titleSegments={getTitleSegments(value.resources)}
        removable
        isProcessing={isProcessing}
        validation={validation}
        onRemove={removeRule}
      >
        <ResourceKindSelect
          components={{ MultiValue: ResourceKindMultiValue }}
          isMulti
          label="Teleport Resources"
          required
          isDisabled={isProcessing}
          options={resourceKindOptions}
          value={resources}
          onChange={setResources}
          rule={precomputed(validation.fields.resources)}
          menuPosition="fixed"
        />
        <VerbEditor
          verbs={verbs}
          allVerbs={allVerbs}
          rule={precomputed(validation.fields.verbs)}
          onVerbChange={setVerbChecked}
          onAllVerbsChange={setAllVerbsChecked}
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
          onChange={e => setWhere(e.target.value)}
          mb={0}
        />
      </SectionBox>
    </ValidationSuspender>
  );
});

function getTitleSegments(resources: readonly ResourceKindOption[]): string[] {
  switch (resources.length) {
    case 0:
      return ['Admin Rule'];
    case 1:
      return ['Admin Rule', resources[0].label];
    default:
      return [
        'Admin Rule',
        `${resources[0].label} + ${resources.length - 1} more`,
      ];
  }
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

/** Renders a grid of allowed permissions (verbs) as a grid of checkboxes. */
function VerbEditor({
  verbs,
  allVerbs,
  rule,
  onVerbChange,
  onAllVerbsChange,
}: {
  verbs: VerbModel[];
  allVerbs: boolean;
  rule: Rule<VerbModel[]>;
  onVerbChange(verb: Verb, checked: boolean): void;
  onAllVerbsChange(checked: boolean): void;
}) {
  const helperTextId = useId();
  const { valid, message } = useRule(rule(verbs));

  // Hardcoded column works here because the editor is fixed-width (defined in
  // the RoleEditorAdapter component).
  const numColumns = verbs.some(
    v => v.verb === 'create_enroll_token' || v.verb === 'readnosecrets'
  )
    ? 2
    : 3;

  return (
    <PermissionsFieldset aria-describedby={helperTextId}>
      <Legend>
        <LabelContent required>Permissions</LabelContent>
      </Legend>
      <FieldCheckbox
        label={'All (wildcard verb “*”)'}
        checked={allVerbs}
        onChange={e => onAllVerbsChange(e.target.checked)}
        mb={0}
      />
      <Divider />
      <PermissionsGrid numColumns={numColumns}>
        {verbs.map(v => (
          <FieldCheckbox
            key={v.verb}
            label={v.verb}
            checked={v.checked}
            onChange={e => onVerbChange(v.verb, e.target.checked)}
            mb={0}
          />
        ))}
      </PermissionsGrid>
      <HelperTextLine
        hasError={!valid}
        helperTextId={helperTextId}
        errorMessage={message}
      />
    </PermissionsFieldset>
  );
}

const PermissionsFieldset = styled.fieldset`
  border: none;
  margin: 0 0 ${props => props.theme.space[3]}px 0;
  padding: 0;
  max-width: 100%;
  box-sizing: border-box;
`;

/**
 * Renders a grid for permissions, using `numColumns` equal-width columns. We
 * need to specify the number of columns explicitly, as `auto-fill` and
 * `auto-fit` can't be mixed with intrinsic or flexible column sizes (see
 * https://drafts.csswg.org/css-grid/#repeat-syntax).
 */
const PermissionsGrid = styled.div<{ numColumns: number }>`
  display: grid;
  grid-template-columns: repeat(${props => props.numColumns}, 1fr);
  row-gap: ${props => props.theme.space[2]}px;
  column-gap: ${props => props.theme.space[3]}px;
`;

const Legend = styled.legend`
  margin: 0 0 ${props => props.theme.space[1]}px 0;
  padding: 0;
  ${props => props.theme.typography.body3}
`;

function Divider() {
  return (
    <Flex
      alignItems="center"
      justifyContent="center"
      flexDirection="column"
      borderBottom={1}
      borderColor="text.muted"
      my={3}
      css={`
        position: relative;
      `}
    >
      <StyledOr>Or</StyledOr>
    </Flex>
  );
}

const StyledOr = styled.div`
  background: ${props => props.theme.colors.levels.surface};
  display: flex;
  align-items: center;
  font-size: 10px;
  height: 32px;
  width: 32px;
  justify-content: center;
  position: absolute;
  text-transform: uppercase;
`;
