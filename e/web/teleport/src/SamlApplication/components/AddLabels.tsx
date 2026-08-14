import { useState } from 'react';

import { Box, Text } from 'design';
import Validation, { Validator } from 'shared/components/Validation';

import type { CreateSamlIdpServiceProviderRequest } from 'e-teleport/services/idp/types';
import { DiscoverLabel, LabelsCreater } from 'teleport/Discover/Shared';

export type AddLabelsProps = {
  spConfig: CreateSamlIdpServiceProviderRequest;
  setSPConfig: (SPConfig) => void;
  setLabelsValidator: (validator: Validator) => void;
  disabled: boolean;
};

/**
 * Allows users to define custom labels for SAML related applications.
 * Supports unguided and generics.
 *
 * Does not yet support adding labels when guided because the
 * "disabled" flag can mean either "is processing" or "is guided".
 *
 * Also does not support adding default labels from guided flows.
 *
 * @param spConfig service provider config includes the label field
 * @param setSPConfig used to update labels in the spConfig
 * @param setLabelsValidator sets the validator to use when validating
 * label fields
 * @param disabled when true, disables inputs and buttons
 */
export function AddLabels({
  spConfig,
  setSPConfig,
  setLabelsValidator,
  disabled,
}: AddLabelsProps) {
  const [labels, setLabels] = useState<DiscoverLabel[]>([]);

  if (
    labels.length === 0 &&
    Object.keys(spConfig.labels).length !== labels.length
  ) {
    const mapOfLabels: DiscoverLabel[] = Object.keys(spConfig.labels).map(
      i => ({ name: i, value: spConfig.labels[i] })
    );
    setLabels(mapOfLabels);
  }

  function onLabelChange(validator: Validator, customLabels: DiscoverLabel[]) {
    setLabelsValidator(validator);
    setLabels(customLabels);
    const mapOfLabels = {};
    customLabels.forEach(label => (mapOfLabels[label.name] = label.value));
    setSPConfig({ ...spConfig, labels: mapOfLabels });
  }

  return (
    <>
      <Text bold>Add Labels (Optional)</Text>
      <Validation>
        {({ validator }) => (
          <Box mb={2} mt={2}>
            <LabelsCreater
              labels={labels}
              setLabels={(l: DiscoverLabel[]) => onLabelChange(validator, l)}
              isLabelOptional={true}
              disableBtns={disabled}
              noDuplicateKey={true}
            />
          </Box>
        )}
      </Validation>
    </>
  );
}
