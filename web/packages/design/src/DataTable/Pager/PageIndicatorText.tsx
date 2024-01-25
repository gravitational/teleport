import React from 'react';

import { Text } from 'design';

export function PageIndicatorText({
  from,
  to,
  count,
}: {
  from: number;
  to: number;
  count: number;
}) {
  if (count == 0) {
    return;
  }

  return (
    <Text
      typography="body2"
      mr={1}
      fontWeight="500"
      style={{
        whiteSpace: 'nowrap',
        letterSpacing: '0.15px',
      }}
    >
      Showing <strong>{from}</strong> - <strong>{to}</strong> of{' '}
      <strong>{count}</strong>
    </Text>
  );
}
