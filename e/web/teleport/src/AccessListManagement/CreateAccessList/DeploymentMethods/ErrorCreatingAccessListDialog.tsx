import { Alert, ButtonPrimary, ButtonSecondary, Flex } from 'design';
import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';

export function ErrorCreatingAccessListDialog({
  retry,
  onCancel,
  error,
}: {
  retry(): void;
  onCancel(): void;
  error: string;
}) {
  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '450px',
        width: '450px',
      })}
      disableEscapeKeyDown={false}
      open={true}
    >
      <DialogHeader>
        <DialogTitle>Failed to create Access List</DialogTitle>
      </DialogHeader>
      <Alert>{error}</Alert>
      <Flex gap={3}>
        <ButtonPrimary onClick={retry} width="100%">
          Retry
        </ButtonPrimary>
        <ButtonSecondary onClick={onCancel} width="100%">
          Cancel
        </ButtonSecondary>
      </Flex>
    </Dialog>
  );
}
