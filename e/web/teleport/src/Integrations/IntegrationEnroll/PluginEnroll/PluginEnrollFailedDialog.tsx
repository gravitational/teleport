import React from 'react';

import { ButtonSecondary, H2, Text } from 'design';
import Dialog, { DialogContent, DialogFooter } from 'design/Dialog';

import { PluginBase } from './plugins';

export function PluginEnrollFailedDialog(props: State) {
  const { plugin, errorDescription, clearError } = props;

  return (
    <Dialog open={true}>
      <DialogContent maxWidth="500px">
        <H2 mb={3}>Unable to connect {plugin.name}</H2>
        <Text>
          <Text typography="h6">Access to {plugin.name} was denied:</Text>
          <Text mono my="2">
            {errorDescription}
          </Text>
        </Text>
        <Text mt="2">
          If you want to try again, close this dialogue and click on "Connect{' '}
          {plugin.name}" button again.
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
  plugin: PluginBase;
  errorDescription: string;
  clearError: () => void;
};
