import { useImperativeHandle, useState } from 'react';
import { ActionMeta, components } from 'react-select';

import { LabelInput, Text } from 'design';
import { FieldSelectCreatable } from 'shared/components/FieldSelect';
import { CustomSelectComponentProps, Option } from 'shared/components/Select';

import { ResourceLabel } from 'teleport/services/agents';
import { Labels } from 'teleport/services/resources';

import { LabelOption } from '../../role/label';
import { wildcard } from '../../role/role';

const wildcardLabelsTxt = 'match by any labels';
const wildcardValueTxt = 'by any value';

export type ResourceLabelInputHandle = {
  onLabelClick: (label: ResourceLabel) => void;
};

/**
 * Converts user input (required format <key: value>) into resource Labels
 * and calls onChange with the updated labels.
 */
export function ResourceLabelInput({
  labels,
  onChange,
  ref,
}: {
  labels: Labels;
  onChange(labels: Labels): void;
  ref: React.Ref<ResourceLabelInputHandle>;
}) {
  const [inputValue, setInputValue] = useState('');
  // User input errors eg: duplicate label or incorrect label format when
  // entering labels manually
  const [error, setError] = useState('');

  const wildcardOption = getWildcardOption(inputValue);
  const selectedLabels = makeLabelOptions(labels);

  function updateLabels(labelOptions: LabelOption[]) {
    const newLabels: Labels = {};
    labelOptions.forEach(opt => {
      newLabels[opt.value.labelKey] = opt.value.labelVals;
    });
    onChange(newLabels);
  }

  function handleWildcard() {
    // Replace all existing labels with wildcard.
    if (wildcardOption.label.includes(wildcardLabelsTxt)) {
      updateLabels([wildcardOption]);
      return;
    }
    if (wildcardOption.label.includes(wildcardValueTxt)) {
      // Remove all other existing values for the matching key.
      if (labels[wildcardOption.value.labelKey]) {
        const newLabels = selectedLabels.filter(
          label => label.value.labelKey !== wildcardOption.value.labelKey
        );
        updateLabels([...newLabels, wildcardOption]);
      } else {
        updateLabels([...selectedLabels, wildcardOption]);
      }
    }
  }

  function handleOnChange(
    selectedOpts: LabelOption[],
    actionMeta: ActionMeta<LabelOption>
  ) {
    const action = actionMeta.action;
    switch (action) {
      case 'create-option':
        const err = validateLabelInput(inputValue);
        setError(err);
        if (err) {
          return;
        }

        addNewLabel(getLabelFromInput(inputValue));
        setInputValue('');
        return;

      case 'select-option':
        const newOpt = selectedOpts.length - 1;
        if (
          selectedOpts[newOpt]?.value.labelVals.length === 1 &&
          selectedOpts[newOpt]?.value.labelVals[0] === wildcard
        ) {
          handleWildcard();
          setError('');
        }
        return;

      // clicking on big "x" (clear all),
      // hitting back button (pop value) or
      // clicking on labels mini "x" button
      // (remove one selected item at a time)
      case 'clear':
      case 'pop-value':
      case 'remove-value':
        updateLabels(selectedOpts);
        setError('');
        return;
    }
  }

  function addNewLabel(label: ResourceLabel) {
    let newLabels = [...selectedLabels];
    let newOption = makeLabelOption(label.name, [label.value]);

    const existingLabelIndex = selectedLabels.findIndex(
      opt => opt.value.labelKey === label.name
    );
    if (existingLabelIndex > -1) {
      const existingLabel = newLabels[existingLabelIndex];
      if (existingLabel.value.labelVals.includes(label.value)) {
        setError(
          `Label "${newOption.label}", value "${label.value}" is already selected`
        );
        return;
      }
      newLabels[existingLabelIndex] = makeLabelOption(label.name, [
        ...existingLabel.value.labelVals,
        label.value,
      ]);
    } else {
      newLabels = [...newLabels, newOption];
    }

    updateLabels(newLabels);
  }

  useImperativeHandle(ref, () => ({
    onLabelClick: (label: ResourceLabel) => {
      setError('');
      addNewLabel(label);
    },
  }));

  return (
    <>
      <FieldSelectCreatable
        data-testid="resource-label-input"
        mb={1}
        css={`
          max-width: 100%;
          ${LabelInput} {
            font-size: ${p => p.theme.fontSizes[2]}px;
            padding-bottom: ${p => p.theme.space[1]}px;
          }
        `}
        components={{
          DropdownIndicator: null,
          MultiValueContainer,
        }}
        value={[...selectedLabels]}
        options={wildcardOption ? [wildcardOption] : undefined}
        filterOption={option => {
          return !!option?.label;
        }}
        inputValue={inputValue}
        onInputChange={setInputValue}
        isMulti
        placeholder="Type a label and press Enter using the format key: value"
        noOptionsMessage={() =>
          'Type a label and press Enter using the format key: value'
        }
        // Hide create label option if a wildcard option exists
        formatCreateLabel={wildcardOption ? () => undefined : undefined}
        onChange={handleOnChange}
        customProps={{
          lastFilter:
            selectedLabels?.length > 1
              ? selectedLabels[selectedLabels.length - 1].label
              : '',
        }}
        autoFocus
      />
      {error && <Text color="interactive.solid.danger.default">{error}</Text>}
    </>
  );
}

function makeLabelOption(labelKey: string, labelVals: string[]) {
  return {
    value: { labelKey, labelVals },
    label: `${labelKey}: ${labelVals.join(' OR ')}`,
  };
}

function makeLabelOptions(labelLookup: Labels): LabelOption[] {
  return Object.keys(labelLookup || {}).map(labelKey => {
    const labelVal = labelLookup[labelKey];
    if (Array.isArray(labelVal)) {
      return makeLabelOption(labelKey, labelVal);
    } else {
      return makeLabelOption(labelKey, [labelVal]);
    }
  });
}

function getLabelFromInput(labelStr: string): ResourceLabel {
  // Split at the first instance of `: ` and then combine the rest
  // back to its original form. It's safe to split with `: ` as backend
  // does not allow a label key to contain a `: `.
  // eg:
  //   - `env: staging` = `labelKey = env` and `labelVal = staging`
  //   - `env: dev: apple` = `labelKey = env` and `labelVal = dev: apple`
  let [labelKey, ...rest] = labelStr.split(': ');
  const labelVal = rest.join(': ');

  return { name: labelKey, value: labelVal };
}

function validateLabelInput(labelInput: string) {
  const { name: key, value } = getLabelFromInput(labelInput);
  if (key.length < 1 || value.length < 1) {
    return `Label "${labelInput}" is invalid. Use label format "key: value" e.g. environment: staging`;
  }
  return '';
}

/**
 * Returns a wildcard option suggestion based on user input.
 */
function getWildcardOption(inputValue: string): LabelOption | undefined {
  const label = getLabelFromInput(inputValue);

  // User is starting label with a wildcard.
  if (label.name === wildcard && (!label.value || label.value === wildcard)) {
    return {
      label: `use wildcard (*) - ${wildcardLabelsTxt}`,
      value: { labelKey: wildcard, labelVals: [wildcard] },
    };
  }

  // User wants to use wildcard as value.
  if (label.value === wildcard && label.name !== wildcard) {
    return {
      label: `use wildcard (*) - match label key "${label.name}" ${wildcardValueTxt}`,
      value: { labelKey: label.name, labelVals: [wildcard] },
    };
  }

  // Suggest a wildcard as a value when value is not defined yet.
  if (
    label.value.length === 0 &&
    label.name !== wildcard &&
    inputValue.includes(': ')
  ) {
    return {
      label: `use wildcard (*) - match label key "${label.name}" ${wildcardValueTxt}`,
      value: { labelKey: label.name, labelVals: [wildcard] },
    };
  }

  return undefined;
}

const MultiValueContainer = (
  props: CustomSelectComponentProps<
    { lastFilter: string },
    Option | LabelOption
  >
) => {
  const lastFilter = props.selectProps.customProps.lastFilter;
  const currFilter = props.data.label;

  const isLastFilter = lastFilter === currFilter;
  return (
    <>
      <components.MultiValueContainer {...props} />
      {lastFilter && !isLastFilter && <Text fontSize={0}>AND</Text>}
    </>
  );
};
