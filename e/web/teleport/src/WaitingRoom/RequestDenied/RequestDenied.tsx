import session from 'teleport/services/websession';
import { ButtonSecondary, Text, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';

export default function RequestDenied({ reason }: Props) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Access Request Denied</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {reason && <Alert kind="danger" children={reason} />}
        <Text mb={3}>
          Your request has been denied. Please contact your administrator for
          more information.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary onClick={() => session.logout()}>
          Logout
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}

type Props = {
  reason: string;
};
