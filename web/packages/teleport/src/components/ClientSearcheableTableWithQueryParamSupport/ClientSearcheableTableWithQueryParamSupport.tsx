/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { useHistory, useLocation } from 'react-router';

import Table from 'design/DataTable';
import { TableProps } from 'design/DataTable/types';

import {
  decodeUrlQueryParam,
  encodeUrlQueryParams,
} from '../hooks/useUrlFiltering';

export function ClientSearcheableTableWithQueryParamSupport<T>(
  props: Omit<TableProps<T>, 'serversideProps'>
) {
  const loc = useLocation();
  const history = useHistory();

  const searchParams = new URLSearchParams(loc.search);

  function updateUrlParams(searchString: string) {
    history.replace(
      encodeUrlQueryParams({ pathname: loc.pathname, searchString })
    );
  }

  return (
    <Table<T>
      {...props}
      clientSearch={{
        initialSearchValue: decodeUrlQueryParam(
          searchParams.get('search') || ''
        ),
        onSearchValueChange: updateUrlParams,
      }}
      isSearchable
    />
  );
}
