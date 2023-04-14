import React from 'react';

import { ButtonSecondary, Text } from 'design';
import Dialog, { DialogContent, DialogFooter } from 'design/Dialog';

import { PluginType } from '../data';

export function PluginEnrollFailedDialog(props: State) {
  const { resolvedType, error, errorDescription, clearError } = props;

  return (
    <Dialog open={!!error}>
      <DialogContent maxWidth="500px">
        <Text typography="h4" fontWeight="bold">
          Unable to connect {resolvedType.name}
        </Text>
        <Text>
          <Text display="inline" typography="h6">
            Access to {resolvedType.name} was denied:
          </Text>
          <Text mono my="2">
            {errorDescription}
          </Text>
        </Text>
        <Text mt="2">
          If you still want to set up the integration, please click the "Connect{' '}
          {resolvedType.name}" button again.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary size="large" width="30%" onClick={clearError}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type State = {
  resolvedType: PluginType;
  error?: string;
  errorDescription?: string;
  clearError: () => void;
};
