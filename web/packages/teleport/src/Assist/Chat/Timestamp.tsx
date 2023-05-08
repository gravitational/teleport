import React, { useEffect, useState } from 'react';

import { formatRelative } from 'date-fns';

interface TimestampProps {
  isoTimestamp: string;
}

export function Timestamp(props: TimestampProps) {
  const [date] = useState(() => new Date(props.isoTimestamp));
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

  return <span title={date.toLocaleString()}>{formatDate(date)}</span>;
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
