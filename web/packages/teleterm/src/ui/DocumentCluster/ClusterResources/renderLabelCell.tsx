import React from 'react';
import { makeLabelTag } from 'teleport/components/formatters';
import { Cell } from 'design/DataTable';
import { Label as SingleLabel } from 'design';
import { Label } from 'teleport/types';

export function renderLabelCell<T extends { labelsList: Label[] }>(props: T) {
  const labels = props.labelsList.map(makeLabelTag);
  const $labels = labels.map(label => (
    <SingleLabel mb="1" mr="1" key={label} kind="secondary">
      {label}
    </SingleLabel>
  ));

  return <Cell>{$labels}</Cell>;
}
