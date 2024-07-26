import { ButtonSecondary, Text, ButtonWarning, Alert } from 'design';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import { Attempt } from 'shared/hooks/useAsync';

export function Delete({
  open,
  onClose,
  appName,
  attempt,
  onDelete,
}: {
  open: boolean;
  onClose?: () => void;
  appName?: string;
  attempt: Attempt<void>;
  onDelete: () => void;
}) {
  return (
    <Dialog
      dialogCss={() => ({ maxWidth: '500px', width: '100%' })}
      disableEscapeKeyDown={false}
      onClose={onClose}
      open={open}
    >
      <DialogHeader>
        <DialogTitle>Delete SAML service provider?</DialogTitle>
      </DialogHeader>
      <DialogContent>
        {attempt.status === 'error' && <Alert children={attempt.statusText} />}
        <Text mb={4}>
          You are about to delete SAML service provider
          <Text bold as="span">
            {` ${appName}`}
          </Text>
          . This will prevent user's access from the application.
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning
          onClick={onDelete}
          mr="3"
          disabled={attempt.status === 'processing'}
        >
          I understand, delete {appName}
        </ButtonWarning>
        <ButtonSecondary onClick={onClose}>Cancel</ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
