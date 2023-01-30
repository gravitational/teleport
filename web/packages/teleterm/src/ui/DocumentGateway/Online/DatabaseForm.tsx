import React, { useCallback, useRef } from 'react';

import { Flex } from 'design';

import Validation from 'shared/components/Validation';

import {
  ConfigFieldInput,
  PortFieldInput,
} from 'teleterm/ui/DocumentGateway/common';

interface DatabaseFormProps {
  dbName: string;
  port: string;
  onDbNameChange: (dbName: string) => void;
  onPortChange: (port: string) => void;
}

export function DatabaseForm(props: DatabaseFormProps) {
  const formRef = useRef<HTMLFormElement>();

  const handleChangePort = useCallback(
    (value: string) => {
      if (formRef.current.reportValidity()) {
        props.onPortChange(value);
      }
    },
    [formRef.current, props.onPortChange]
  );

  return (
    <Flex as="form" ref={formRef}>
      <Validation>
        <PortFieldInput
          label="Port"
          defaultValue={props.port}
          onChange={e => handleChangePort(e.target.value)}
          mb={2}
        />
        <ConfigFieldInput
          label="Database name"
          defaultValue={props.dbName}
          onChange={e => props.onDbNameChange(e.target.value)}
          spellCheck={false}
          ml={2}
          mb={2}
        />
      </Validation>
    </Flex>
  );
}
