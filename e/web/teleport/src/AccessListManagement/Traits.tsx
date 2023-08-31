import React from 'react';
import { Box, Flex, ButtonIcon, Text, ButtonBorder } from 'design';
import * as Icons from 'design/Icon';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';
import { AllUserTraits } from 'teleport/services/user';

import { EditKind } from './Shared';

export function TraitsCreator({
  traitLabels = [],
  updateTraitLabels,
  isDisabled = false,
  autoFocus = false,
  kind,
}: {
  traitLabels: TraitLabel[];
  updateTraitLabels(l: TraitLabel[]): void;
  isDisabled?: boolean;
  autoFocus?: boolean;
  kind: EditKind;
}) {
  function addLabel() {
    updateTraitLabels([...traitLabels, { name: '', value: '' }]);
  }

  function removeLabel(index: number) {
    const newList = [...traitLabels];
    newList.splice(index, 1);
    updateTraitLabels(newList);
  }

  const handleChange = (
    event: React.ChangeEvent<HTMLInputElement>,
    index: number,
    labelField: keyof TraitLabel
  ) => {
    const { value } = event.target;
    const newList = [...traitLabels];

    newList[index] = { ...newList[index], [labelField]: value };
    updateTraitLabels(newList);
  };

  let addBtnTxt =
    traitLabels.length === 0
      ? `Add ${kind} Required Trait`
      : `Add Another Required ${kind} Trait`;

  if (kind === 'Grants') {
    addBtnTxt =
      traitLabels.length === 0
        ? `Add Trait to Grant`
        : `Add Another Trait to Grant`;
  }

  return (
    <Box mb={4}>
      <ButtonBorder
        size="small"
        onClick={addLabel}
        css={`
          padding-right: 20px;
          &:disabled {
            .icon-add {
              opacity: 0.35;
            }
            pointer-events: none;
          }
        `}
        disabled={isDisabled}
      >
        <Icons.Add
          className="icon-add"
          disabled={isDisabled}
          size={12}
          css={`
            margin-right: 3px;
          `}
        />
        {addBtnTxt}
      </ButtonBorder>
      {traitLabels.length > 0 && (
        <Flex mt={2}>
          <Box width="186px">
            <Text mr="3" fontSize={1}>
              Key (required)
            </Text>
          </Box>
          <Text fontSize={1}>Value (required)</Text>
        </Flex>
      )}
      <Box>
        {traitLabels.map((label, index) => {
          return (
            <Box mb={2} key={index}>
              <Flex alignItems="center">
                <FieldInput
                  Input
                  rule={requiredField('required')}
                  autoFocus={autoFocus}
                  value={label.name}
                  placeholder="trait key"
                  width="170px"
                  mr={3}
                  mb={0}
                  onChange={e => handleChange(e, index, 'name')}
                  readonly={isDisabled}
                />
                <FieldInput
                  rule={requiredField('required')}
                  value={label.value}
                  placeholder="trait value"
                  width="170px"
                  mb={0}
                  mr={2}
                  onChange={e => handleChange(e, index, 'value')}
                  readonly={isDisabled}
                />
                <ButtonIcon
                  size={1}
                  title="Remove Label"
                  onClick={() => removeLabel(index)}
                  css={`
                    &:disabled {
                      opacity: 0.65;
                      pointer-events: none;
                    }
                  `}
                  disabled={isDisabled}
                >
                  <Icons.Trash size="medium" />
                </ButtonIcon>
              </Flex>
            </Box>
          );
        })}
      </Box>
    </Box>
  );
}

export type TraitLabel = {
  name: string;
  value: string;
};

// TraitLookup is a map of trait keys to a map of trait values.
export type TraitLookup = Record<string, Record<string, boolean>>;

// TraitConvenience provides convenient data formats for traits
// to be used for:
//    - rendering trait creator/editor
//    - calculating eligible users by looking up traits
//    - rendering list of traits
export type TraitConvenience = {
  traitLabels: TraitLabel[];
  traitLookup: TraitLookup;
  traitList: string[];
};

export function deduplicateTraitLabels(labels: TraitLabel[]): TraitLabel[] {
  const seen: Record<string, string[]> = {};
  return labels.filter(label => {
    if (seen[label.name]) {
      if (seen[label.name].includes(label.value)) {
        return false;
      }
      seen[label.name].push(label.value);
      return true;
    }
    seen[label.name] = [label.value];
    return true;
  });
}

export function convertTraitLabelsToAllUserTraits(
  labels: TraitLabel[]
): AllUserTraits {
  const traits: AllUserTraits = {};

  deduplicateTraitLabels(labels).forEach(label => {
    if (traits[label.name]) {
      traits[label.name].push(label.value);
      return;
    }
    traits[label.name] = [label.value];
  });

  return traits;
}

export function convertTraitLabelsToTraitLookup(
  labels: TraitLabel[]
): TraitLookup {
  const traitLookup: TraitLookup = {};

  labels.forEach(label => {
    if (traitLookup[label.name]) {
      // Effectively removes duplicates.
      traitLookup[label.name][label.value] = true;
      return;
    }
    traitLookup[label.name] = { [label.value]: true };
  });

  return traitLookup;
}

export function convertToTraitConvenience(
  traits: AllUserTraits
): TraitConvenience {
  const traitLabels: TraitLabel[] = [];
  const traitList: string[] = [];

  Object.keys(traits).forEach(traitKey => {
    if (traits[traitKey] && traits[traitKey].length > 0) {
      const traitVals = traits[traitKey];
      traitVals.forEach(val =>
        traitLabels.push({ name: traitKey, value: val })
      );
      traitList.push(`${traitKey}: ${traitVals.sort().join(', ')}`);
    }
  });

  return {
    traitLabels,
    traitList,
    traitLookup: convertTraitLabelsToTraitLookup(traitLabels),
  };
}
