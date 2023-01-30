import React from 'react';

import { Flex } from 'design';

import * as Alerts from 'design/Alert';
import { Attempt } from 'shared/hooks/useAsync';

interface ErrorsProps {
  dbNameAttempt: Attempt<void>;
  portAttempt: Attempt<void>;
}

export function Errors(props: ErrorsProps) {
  return (
    <Flex flexDirection="column" gap={2} mb={3}>
      {props.dbNameAttempt.status === 'error' && (
        <Alerts.Danger mb={0}>
          Could not change the database name: {props.dbNameAttempt.statusText}
        </Alerts.Danger>
      )}
      {props.portAttempt.status === 'error' && (
        <Alerts.Danger mb={0}>
          Could not change the port number: {props.portAttempt.statusText}
        </Alerts.Danger>
      )}
    </Flex>
  );
}
