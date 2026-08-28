import { Alert, Box, ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import Dialog, { DialogHeader, DialogTitle } from 'design/Dialog';
import { TextSelectCopyMulti } from 'shared/components/TextSelectCopy';

import { PendingCleanup } from '../presetcleanup';

export function ErrorCreatingAccessListDialog({
  retry,
  onCancel,
  error,
  requiresCleanup,
}: {
  retry(): void;
  onCancel(): void;
  error: string;
  requiresCleanup?: PendingCleanup;
}) {
  const cmds: string[] = [];
  if (requiresCleanup?.error) {
    if (requiresCleanup?.accessListId) {
      cmds.push(`tctl acl rm ${requiresCleanup.accessListId}`);
    }
    for (const role of requiresCleanup?.roles ?? []) {
      cmds.push(`tctl rm role/${role}`);
    }
  }
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
      {error && <Alert>{error}</Alert>}
      {requiresCleanup?.error && (
        <Box mb={4}>
          {!error && <Alert>{requiresCleanup.error}</Alert>}
          <Text>
            Could not remove leftover resources from the previous attempt,
            either retry or try the following command on your CLI:
          </Text>
          {cmds.length > 0 && (
            <TextSelectCopyMulti lines={cmds.map(text => ({ text }))} />
          )}
        </Box>
      )}
      <Flex gap={3}>
        <ButtonPrimary onClick={retry} width="100%">
          Retry
        </ButtonPrimary>
        <ButtonSecondary onClick={onCancel} width="100%">
          {requiresCleanup?.error ? 'Dismiss' : 'Cancel'}
        </ButtonSecondary>
      </Flex>
    </Dialog>
  );
}
