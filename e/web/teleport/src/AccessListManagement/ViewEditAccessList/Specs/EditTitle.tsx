import { useState } from 'react';

import { Alert, Box, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import Validation, { Validator } from 'shared/components/Validation';
import { requiredField } from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';

import {
  AccessList,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';

import { AccessListModified } from '../Shared';

export function EditTitle({
  onClose,
  accessList,
  updateAccessList,
}: {
  onClose(): void;
  accessList: AccessListModified;
  updateAccessList(accessList: AccessList): void;
}) {
  const { title: originalTitle } = accessList;
  const { attempt, setAttempt } = useAttempt('');
  const [newTitle, setNewTitle] = useState(originalTitle);

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }
    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .updateAccessList({
        req: { title: newTitle },
        original: accessList,
      })
      .then(resp => {
        onClose();
        updateAccessList(resp);
      })
      .catch((e: Error) =>
        setAttempt({ status: 'failed', statusText: e.message })
      );
  }

  return (
    <Validation>
      {({ validator }) => (
        <Dialog
          dialogCss={() => ({
            maxWidth: '500px',
            width: '100%',
          })}
          disableEscapeKeyDown={false}
          onClose={onClose}
          open={true}
        >
          <DialogHeader>
            <DialogTitle>Edit Title</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <Box>
              <FieldInput
                mr={2}
                label="Title"
                rule={requiredField('Title is required')}
                placeholder="title"
                autoFocus
                value={newTitle}
                onChange={e => setNewTitle(e.target.value)}
                disabled={attempt.status === 'processing'}
              />
            </Box>
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={attempt.status === 'processing'}
              onClick={() => handleOnCreate(validator)}
            >
              Edit Title
            </ButtonPrimary>
            <ButtonSecondary
              disabled={attempt.status === 'processing'}
              onClick={onClose}
            >
              Cancel
            </ButtonSecondary>
          </DialogFooter>
        </Dialog>
      )}
    </Validation>
  );
}
