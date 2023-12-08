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

import React, { useEffect, useState } from 'react';

import { formatRelative } from 'date-fns';

interface TimestampProps {
  timestamp: Date;
}

export function Timestamp(props: TimestampProps) {
  const [, setCounter] = useState(0);

  useEffect(() => {
    const id = window.setInterval(
      () => setCounter(count => count + 1),
      1000 * 60
    );

    return () => {
      clearInterval(id);
    };
  }, []);

  return (
    <span title={props.timestamp.toLocaleString()}>
      {formatDate(props.timestamp)}
    </span>
  );
}

function formatDate(date: Date) {
  const now = Date.now();
  const compare = date.getTime();

  if (now - compare < 1000 * 60) {
    return 'just now';
  }

  const minutes = Math.floor((now - compare) / 60000);

  if (minutes === 1) {
    return 'a minute ago';
  }

  if (minutes > 59 && minutes < 120) {
    return 'an hour ago';
  }

  if (minutes >= 120) {
    const hours = Math.floor(minutes / 60);

    if (hours >= 24) {
      return formatRelative(date, Date.now());
    }

    return `${hours} hours ago`;
  }

  return `${minutes} minutes ago`;
}
