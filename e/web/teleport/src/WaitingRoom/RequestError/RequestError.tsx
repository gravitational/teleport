import React from 'react';
import session from 'teleport/services/websession';
import { ButtonSecondary, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

export default function RequestError({ err }: Props) {
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
        <Text mb={3}>Please try again by refreshing the page.</Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={() => session.logout()}>
          {`Cancel & Logout`}
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  err: string;
};
