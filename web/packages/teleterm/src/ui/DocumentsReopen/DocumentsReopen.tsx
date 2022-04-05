import React from 'react';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { ButtonIcon, ButtonPrimary, ButtonSecondary, Text } from 'design';
import { Close } from 'design/Icon';

interface DocumentsReopenProps {
  onCancel(): void;

  onConfirm(): void;
}

export function DocumentsReopen(props: DocumentsReopenProps) {
  return (
    <DialogConfirmation
      open={true}
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          props.onConfirm();
        }}
      >
        <DialogHeader
          justifyContent="space-between"
          mb={0}
          alignItems="baseline"
        >
          <Text typography="h4" bold>
            Reopen previous session
          </Text>
          <ButtonIcon
            type="button"
            onClick={props.onCancel}
            color="text.secondary"
          >
            <Close fontSize={5} />
          </ButtonIcon>
        </DialogHeader>
        <DialogContent mb={4}>
          <Text typography="body1" color="text.secondary">
            Do you want to reopen tabs from the previous session?
          </Text>
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary autoFocus mr={3} type="submit">
            Reopen
          </ButtonPrimary>
          <ButtonSecondary type="button" onClick={props.onCancel}>
            Start new session
          </ButtonSecondary>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}
