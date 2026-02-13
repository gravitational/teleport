/*
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import Flex from 'design/Flex';
import {
  MultiselectMenu,
  Option,
} from 'shared/components/Controls/MultiselectMenu';

type Filter<Item, Value> = {
  options: readonly Option<Value>[];
  selected: Value[];
  apply: (l: Item[], s: Value[]) => Item[];
};

// FilterMap gives readable structure to our input list of filters, i.e.:
// {
//   Tags: {
//     options: [{label: "Bot", value: "bot"}, {label: "CI/CD", name: "cicd"}],
//     selected: ["bot"],
//     apply: (list, selected) => list.filter(i => i.tags.includes(selected));
//   }
// }
export type FilterMap<Item, Values extends Record<string, unknown>> = {
  [K in keyof Values]: Filter<Item, Values[K]>;
};

type Props<Item, Values extends Record<string, unknown>> = {
  filters: FilterMap<Item, Values>;
  onFilterChange: (filters: FilterMap<Item, Values>) => void;
};

// ListFilters manages multiple filter dropdowns associated with
// a list of data.
export function ListFilters<Item, Values extends Record<string, unknown>>(
  props: Props<Item, Values>
) {
  const handleFilterChange = <K extends keyof Values>(
    name: K,
    opts: Values[K][]
  ) => {
    props.onFilterChange({
      ...props.filters,
      [name]: {
        ...props.filters[name],
        selected: opts,
      },
    });
  };

  return (
    <Flex gap={2}>
      {Object.entries(props.filters).map(([name, filter]) => (
        <MultiselectMenu
          key={name}
          options={filter.options}
          selected={filter.selected}
          onChange={opts => handleFilterChange(name, opts)}
          label={name}
          tooltip={`Filter by ${name}`}
        />
      ))}
    </Flex>
  );
}
