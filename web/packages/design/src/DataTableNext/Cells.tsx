import React from 'react';
import { Label } from 'design';
import * as Icons from 'design/Icon';
import { displayDate } from 'shared/services/loc';
import { SortDir } from './types';

export const Cell = props => <td children={props.children} {...props} />;

export const SortHeaderCell = ({ dir, text, onClick }: SortHeaderCellProps) => {
  return (
    <th>
      <a onClick={onClick}>
        {text}
        <SortIndicator sortDir={dir} />
      </a>
    </th>
  );
};

export const SortIndicator = ({
  sortDir,
}: {
  sortDir?: SortHeaderCellProps['dir'];
}) => {
  if (sortDir === 'DESC') {
    return <Icons.SortDesc />;
  }

  if (sortDir === 'ASC') {
    return <Icons.SortAsc />;
  }

  return <Icons.Sort />;
};

export const TextCell = ({ data }) => <Cell>{`${data || ''}`}</Cell>;

export const LabelCell = ({ data }: { data: string[] }) =>
  renderLabelCell(data);

export const DateCell = ({ data }: { data: Date }) => (
  <Cell>{displayDate(data)}</Cell>
);

const renderLabelCell = (labels: string[] = []) => {
  const $labels = labels.map(label => (
    <Label mb="1" mr="1" key={label} kind="secondary">
      {label}
    </Label>
  ));

  return <Cell>{$labels}</Cell>;
};

type SortHeaderCellProps = {
  text: string;
  dir: SortDir;
  onClick: () => void;
};
