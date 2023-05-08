import React, { useEffect, useState } from 'react';

export function Timestamp() {
  const [date] = useState(() => new Date());
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

  return <span>{formatDate(date)}</span>;
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

  return `${minutes} minutes ago`;
}
