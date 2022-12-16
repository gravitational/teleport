import React from 'react';
import { ButtonSecondary, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import useWebSession from 'teleport/useWebSession';

export default function RequestError({ err }: Props) {
  const webSession = useWebSession();

  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>An Error has Occurred</DialogTitle>
      </DialogHeader>
      <DialogContent>
        <Alert kind="danger" children={err} />
        <Text mb={3}>Please try again by refreshing page.</Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={() => webSession.logout()}>
          {`Cancel & Logout`}
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  err: string;
};
