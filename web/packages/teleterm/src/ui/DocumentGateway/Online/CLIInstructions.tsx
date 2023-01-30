import React, { useCallback, useRef } from 'react';

import { Flex, Text } from 'design';
import Validation from 'shared/components/Validation';

import {
  ConfigFieldInput,
  PortFieldInput,
} from 'teleterm/ui/DocumentGateway/common';
import { CliCommand } from 'teleterm/ui/DocumentGateway/CliCommand';

import type { Gateway } from 'teleterm/services/tshd/types';

interface CLIInstructionsProps {
  gateway: Gateway;
  isProcessing: boolean;
  onChangePort: (value: string) => void;
  onChangeDbName: (value: string) => void;
  onRunCommand: () => void;
}

export function CLIInstructions(props: CLIInstructionsProps) {
  const formRef = useRef<HTMLFormElement>();

  const handleChangePort = useCallback(
    (value: string) => {
      if (formRef.current.reportValidity()) {
        props.onChangePort(value);
      }
    },
    [formRef.current, props.onChangePort]
  );

  return (
    <>
      <Text typography="h4" mb={1}>
        Connect with CLI
      </Text>
      <Flex as="form" ref={formRef}>
        <Validation>
          <PortFieldInput
            label="Port"
            defaultValue={props.gateway.localPort}
            onChange={e => handleChangePort(e.target.value)}
            mb={2}
          />
          <ConfigFieldInput
            label="Database name"
            defaultValue={props.gateway.targetSubresourceName}
            onChange={e => props.onChangeDbName(e.target.value)}
            spellCheck={false}
            ml={2}
            mb={2}
          />
        </Validation>
      </Flex>
      <CliCommand
        cliCommand={props.gateway.cliCommand}
        isLoading={props.isProcessing}
        onRun={props.onRunCommand}
      />
    </>
  );
}
