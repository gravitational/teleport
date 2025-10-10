import React from 'react';

import { Box, ButtonIcon, Flex, Text } from 'design';
import * as Icons from 'design/Icon';
import { inputGeometry } from 'design/Input/Input';
import { ButtonWithAddIcon } from 'shared/components/ButtonWithAddIcon';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';

import { AllUserTraits } from 'teleport/services/user';

import { EditKind } from './Shared/Shared';

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
      ? `Add a Required Trait (Optional)`
      : `Add Another Required ${kind} Trait`;

  if (kind === 'Grants') {
    addBtnTxt =
      traitLabels.length === 0
        ? `Add a Trait to Grant`
        : `Add Another Trait to Grant`;
  }

  const inputSize = 'medium';
  return (
    <Box mb={4} mt={-2}>
      <ButtonWithAddIcon
        label={addBtnTxt}
        onClick={addLabel}
        disabled={isDisabled}
        iconSize={12}
      />
      {traitLabels.length > 0 && (
        <Flex mt={2}>
          <Box width="186px">
            <Text mr="3" typography="body3">
              Key (required)
            </Text>
          </Box>
          <Text typography="body3">Value (required)</Text>
        </Flex>
      )}
      <Box>
        {traitLabels.map((label, index) => {
          return (
            <Box mb={2} key={index}>
              <Flex alignItems="start">
                <FieldInput
                  size={inputSize}
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
                  size={inputSize}
                  rule={requiredField('required')}
                  value={label.value}
                  placeholder="trait value"
                  width="170px"
                  mb={0}
                  mr={2}
                  onChange={e => handleChange(e, index, 'value')}
                  readonly={isDisabled}
                />
                {/* Force the trash button container to be the same height as an
                    input. We can't just set `alignItems="center"` on the parent
                    flex container above, because the field can expand when
                    showing a validation error. */}
                <Flex
                  alignItems="center"
                  height={inputGeometry[inputSize].height}
                >
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

// TraitConvenience provides convenient data formats for traits
// to be used for:
//    - rendering trait creator/editor
//    - calculating eligible users by looking up traits
//    - rendering list of traits
export type TraitConvenience = {
  traitLabels: TraitLabel[];
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
      traitList.push(makeTraitLabel(traitKey, traitVals));
    }
  });

  return {
    traitLabels,
    traitList,
  };
}

export function makeTraitLabel(traitKey: string, traitVals: string[]) {
  return `${traitKey}: ${traitVals.sort().join(', ')}`;
}
