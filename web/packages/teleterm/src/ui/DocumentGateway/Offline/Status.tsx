import React from 'react';

import { Text } from 'design';

import * as Alerts from 'design/Alert';

import LinearProgress from 'teleterm/ui/components/LinearProgress';

import type { Attempt } from 'shared/hooks/useAsync';

interface StatusProps {
  attempt: Attempt<void>;
}

export function Status(props: StatusProps) {
  const statusDescription =
    props.attempt.status === 'processing' ? 'being set up' : 'offline';

  return (
    <>
      <Text
        typography="h5"
        color="text.primary"
        mb={2}
        style={{ position: 'relative' }}
      >
        The database connection is {statusDescription}
        {props.attempt.status === 'processing' && <LinearProgress />}
      </Text>

      {props.attempt.status === 'error' && (
        <Alerts.Danger mb={0}>{props.attempt.statusText}</Alerts.Danger>
      )}
    </>
  );
}
