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

import { useEffect, useState } from 'react';

export default function usePages({ pageSize, data }) {
  const [startFrom, setFrom] = useState(0);

  // set current page to 0 when data source length changes
  useEffect(() => {
    setFrom(0);
  }, [data.length]);

  function onPrev() {
    let prevPage = startFrom - pageSize;
    if (prevPage < 0) {
      prevPage = 0;
    }
    setFrom(prevPage);
  }

  function onNext() {
    let nextPage = startFrom + pageSize;
    if (nextPage < data.length) {
      nextPage = startFrom + pageSize;
      setFrom(nextPage);
    }
  }

  const totalRows = data.length;

  let endAt = 0;
  let pagedData = data;

  if (data.length > 0) {
    endAt = startFrom + (pageSize > data.length ? data.length : pageSize);
    if (endAt > data.length) {
      endAt = data.length;
    }

    pagedData = data.slice(startFrom, endAt);
  }

  return {
    pageSize,
    startFrom,
    endAt,
    totalRows,
    data: pagedData,
    hasPages: totalRows > pageSize,
    onNext,
    onPrev,
  };
}
