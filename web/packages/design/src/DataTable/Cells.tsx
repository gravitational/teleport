/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { TdHTMLAttributes } from 'react';

import { Theme } from 'design/theme';

import { displayDate } from '../datetime';
import Flex from '../Flex';
import * as Icons from '../Icon';
import Label from '../Label';
import {
  LabelDescription,
  ServersideProps,
  SortDir,
  TableColumn,
} from './types';

export function Cell(props: TdHTMLAttributes<HTMLTableCellElement>) {
  return <td {...props} />;
}

export function SortHeaderCell<T>({
  column,
  serversideProps,
  dir,
  text,
  onClick,
}: SortHeaderCellProps<T>) {
  function handleServersideClick() {
    serversideProps?.setSort({
      dir: serversideProps.sort?.dir === 'ASC' ? 'DESC' : 'ASC',
      // @ts-expect-error TODO(gzdunek): The key can be undefined since the column can provide altKey. Improve the types.
      fieldName: column.key,
    });
  }

  if (serversideProps) {
    return (
      <th>
        <a
          onClick={handleServersideClick}
          style={{ display: 'flex', alignItems: 'center' }}
        >
          {text}
          <SortIndicator
            sortDir={
              serversideProps.sort?.fieldName === column.key
                ? serversideProps.sort.dir
                : undefined
            }
          />
        </a>
      </th>
    );
  }

  return (
    <th>
      <a onClick={onClick} style={{ display: 'flex', alignItems: 'center' }}>
        {text}
        <SortIndicator sortDir={dir} />
      </a>
    </th>
  );
}

export function SortIndicator<T>({
  sortDir,
}: {
  sortDir?: SortHeaderCellProps<T>['dir'];
}) {
  if (sortDir === 'DESC') {
    return <Icons.ChevronDown title="sort items desc" />;
  }

  if (sortDir === 'ASC') {
    return <Icons.ChevronUp title="sort items asc" />;
  }

  return <Icons.ChevronsVertical title="sort items" />;
}

export const TextCell = ({ data }: { data: unknown }) => (
  <Cell>{`${data || ''}`}</Cell>
);

export const LabelCell = ({ data }: { data: string[] }) =>
  renderLabelCell(data);

export const DateCell = ({ data }: { data: Date }) => (
  <Cell>{displayDate(data)}</Cell>
);

const renderLabelCell = (labels: string[] = []) => {
  const $labels = labels.map((label, index) => (
    <Label mr="1" key={`${label}${index}`} kind="secondary">
      {label}
    </Label>
  ));

  return (
    <Cell>
      <Flex flexWrap="wrap">{$labels}</Flex>
    </Cell>
  );
};

export const ClickableLabelCell = ({
  labels,
  onClick,
}: {
  labels: LabelDescription[];
  onClick: (label: LabelDescription) => void;
}) => {
  const $labels = labels.map((label, index) => {
    const labelText = `${label.name}: ${label.value}`;

    return (
      <Label
        onClick={() => onClick(label)}
        key={`${label.name}${label.value}${index}`}
        mr="1"
        kind="secondary"
        css={`
          cursor: pointer;
          &:hover {
            background-color: ${(props: { theme: Theme }) =>
              props.theme.colors.spotBackground[1]};
          }
        `}
      >
        {labelText}
      </Label>
    );
  });

  return (
    <Cell>
      <Flex flexWrap="wrap">{$labels}</Flex>
    </Cell>
  );
};

type SortHeaderCellProps<T> = {
  column: TableColumn<T>;
  serversideProps?: ServersideProps;
  text: string;
  dir?: SortDir;
  onClick: () => void;
};
