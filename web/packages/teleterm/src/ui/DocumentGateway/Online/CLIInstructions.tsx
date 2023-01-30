import React from 'react';
import { Text } from 'design';

export function CLIInstructions(props: React.PropsWithChildren<unknown>) {
  return (
    <>
      <Text typography="h4" mb={1}>
        Connect with CLI
      </Text>

      {props.children}
    </>
  );
}
