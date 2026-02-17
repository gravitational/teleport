import { useCallback, useState, type ReactNode } from 'react';

import { ButtonSecondary, ButtonWarning } from 'design/Button';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';

interface DeleteButtonProps {
  buttonText: string;
  confirmText: ReactNode;
  headerText: string;
  isPending: boolean;
  onDelete: () => Promise<void>;
}

export function DeleteButton({
  buttonText,
  confirmText,
  headerText,
  isPending,
  onDelete,
}: DeleteButtonProps) {
  const [isOpen, setIsOpen] = useState(false);

  const handleDelete = useCallback(async () => {
    await onDelete();

    setIsOpen(false);
  }, [onDelete]);

  return (
    <>
      <ButtonWarning type="button" onClick={() => setIsOpen(true)}>
        {buttonText}
      </ButtonWarning>

      <Dialog
        dialogCss={() => ({
          maxWidth: '400px',
          width: '100%',
        })}
        disableEscapeKeyDown={false}
        disableBackdropClick={false}
        onClose={() => setIsOpen(false)}
        open={isOpen}
      >
        <DialogHeader>
          <DialogTitle>{headerText}</DialogTitle>
        </DialogHeader>

        <DialogContent>{confirmText}</DialogContent>

        <DialogFooter>
          <ButtonWarning
            disabled={isPending}
            onClick={() => {
              void handleDelete();
            }}
            mr={3}
          >
            Yes, Delete
          </ButtonWarning>
          <ButtonSecondary onClick={() => setIsOpen(false)}>
            Cancel
          </ButtonSecondary>
        </DialogFooter>
      </Dialog>
    </>
  );
}
