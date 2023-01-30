import React, { useState } from 'react';
import Validation from 'shared/components/Validation';

import { ButtonPrimary, Flex } from 'design';

import { PortFieldInput } from 'teleterm/ui/DocumentGateway/common';

interface ReconnectFormProps {
  onSubmit: (port: string) => void;
  port: string;
  showPortInput: boolean;
  disabled: boolean;
}

export function ReconnectForm(props: ReconnectFormProps) {
  const [port, setPort] = useState(props.port);

  return (
    <Flex
      as="form"
      onSubmit={() => props.onSubmit(port)}
      alignItems="flex-end"
      flexWrap="wrap"
      justifyContent="space-between"
      mt={3}
      gap={2}
    >
      {props.showPortInput && (
        <Validation>
          <PortFieldInput
            label="Port (optional)"
            value={port}
            mb={0}
            readonly={props.disabled}
            onChange={e => setPort(e.target.value)}
          />
        </Validation>
      )}
      <ButtonPrimary type="submit" disabled={props.disabled}>
        Reconnect
      </ButtonPrimary>
    </Flex>
  );
}
