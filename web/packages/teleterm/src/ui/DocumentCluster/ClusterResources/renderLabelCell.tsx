import React from 'react';

import { makeLabelTag } from 'teleport/components/formatters';
import { Cell } from 'design/DataTable';
import { Label, Flex } from 'design';
import * as types from 'teleport/types';

export function renderLabelCell<T extends { labelsList: types.Label[] }>(
  props: T
) {
  const labels = props.labelsList.map(makeLabelTag);
  const $labels = labels.map(label => (
    <Label key={label} kind="secondary">
      {label}
    </Label>
  ));

  return (
    <Cell>
      <Flex flexWrap="wrap" gap={1}>
        {$labels}
      </Flex>
    </Cell>
  );
}
