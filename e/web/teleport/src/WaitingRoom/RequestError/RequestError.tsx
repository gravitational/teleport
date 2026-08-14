import { Alert, ButtonSecondary, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

import session from 'teleport/services/websession';

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
