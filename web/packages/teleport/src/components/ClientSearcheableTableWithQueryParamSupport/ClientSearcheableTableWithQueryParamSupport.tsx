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

import { useHistory } from 'react-router';

import { TableProps } from 'design/DataTable/types';
import Table from 'design/DataTable';

import {
  decodeUrlQueryParam,
  encodeUrlQueryParams,
} from '../hooks/useUrlFiltering';

export function ClientSearcheableTableWithQueryParamSupport<T>(
  props: TableProps<T>
) {
  const searchParams = new URLSearchParams(location.search);
  const history = useHistory();

  function updateUrlParams(searchString: string) {
    history.replace(
      encodeUrlQueryParams({ pathname: location.pathname, searchString })
    );
  }

  return (
    <Table<T>
      {...props}
      clientSearchConfig={{
        initialSearchValue: decodeUrlQueryParam(
          searchParams.get('search') || ''
        ),
        updateUrlQueryParams: updateUrlParams,
      }}
      isSearchable
    />
  );
}
