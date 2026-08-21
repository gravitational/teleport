import { Alert } from 'design/Alert/Alert';
import { ButtonSecondary, ButtonWarning } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import Flex from 'design/Flex';
import { P } from 'design/Text/Text';

/** Deactivating re-opens the cluster to every IP address, so it is confirmed. */
export function DeactivateDialog({
  onConfirm,
  onCancel,
  processing,
  error,
}: {
  onConfirm: () => void;
  onCancel: () => void;
  processing: boolean;
  error?: string;
}) {
  return (
    <Dialog onClose={onCancel} open={true}>
      <DialogHeader>
        <DialogTitle>Deactivate IP allowlist?</DialogTitle>
      </DialogHeader>
      <DialogContent maxWidth={540}>
        {error && <Alert kind="danger">{error}</Alert>}
        <P>
          Deactivating stops enforcement and re-opens the cluster to every IP
          address. Your allowlist is kept and moves back to draft, so you can
          re-apply it at any time.
        </P>
      </DialogContent>
      <DialogFooter>
        <Flex gap={3}>
          <ButtonWarning disabled={processing} onClick={onConfirm}>
            Deactivate
          </ButtonWarning>
          <ButtonSecondary disabled={processing} onClick={onCancel}>
            Cancel
          </ButtonSecondary>
        </Flex>
      </DialogFooter>
    </Dialog>
  );
}
