import { ButtonPrimary, ButtonSecondary, Flex, Text } from 'design';
import Dialog, { DialogContent } from 'design/Dialog';

/**
 * Notify user that there were no resource access defined when user tries
 * to go to the next step in the flow.
 */
export function NoAccessDefinedDialog({
  onCancel,
  onNext,
}: {
  onCancel(): void;
  onNext(): void;
}) {
  return (
    <Dialog
      dialogCss={() => ({
        maxWidth: '350px',
      })}
      disableEscapeKeyDown={false}
      open={true}
    >
      <DialogContent>
        <Text>
          No resource access is defined. You can still create an access list and
          define the access later by going back to edit this access list.
        </Text>
      </DialogContent>
      <Flex gap={3}>
        <ButtonPrimary onClick={() => onNext()} width="100%">
          Next
        </ButtonPrimary>
        <ButtonSecondary onClick={onCancel} width="100%">
          Cancel
        </ButtonSecondary>
      </Flex>
    </Dialog>
  );
}
