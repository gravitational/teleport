import React from 'react';

import { Text, ButtonIcon, ButtonWarning } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Close } from 'design/Icon';

const changeSelectedClusterWarning =
  'Resources from different clusters cannot be combined in an access request. Current items selected will be cleared. Are you sure you want to continue?';

export default function ConfirmClusterChangeDialog({
  confirmChangeTo,
  onClose,
  onConfirm,
}: Props) {
  return (
    <DialogConfirmation
      open={!!confirmChangeTo}
      onClose={onClose}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" mb={0}>
        <Text typography="h5" bold style={{ whiteSpace: 'nowrap' }}>
          Change clusters?
        </Text>
        <ButtonIcon onClick={onClose} color="text.secondary">
          <Close fontSize={5} />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent mb={4}>
        <Text color="text.secondary" typography="body1">
          {changeSelectedClusterWarning}
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning size="large" block={true} onClick={() => onConfirm()}>
          Confirm
        </ButtonWarning>
      </DialogFooter>
    </DialogConfirmation>
  );
}

type Props = {
  confirmChangeTo: string;
  onClose: () => void;
  onConfirm: () => void;
};
