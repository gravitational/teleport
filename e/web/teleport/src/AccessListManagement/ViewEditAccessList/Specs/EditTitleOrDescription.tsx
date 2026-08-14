import { useState } from 'react';

import { Alert, Box, ButtonPrimary, ButtonSecondary } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/Dialog';
import FieldInput from 'shared/components/FieldInput';
import { FieldTextArea } from 'shared/components/FieldTextArea';
import Validation, { Validator } from 'shared/components/Validation';
import {
  requiredField,
  requiredMaxLength,
} from 'shared/components/Validation/rules';
import useAttempt from 'shared/hooks/useAttemptNext';

import { TextEditKind } from 'e-teleport/AccessListManagement/Shared/Shared';
import {
  AccessList,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';

import { AccessListModified } from '../Shared';

export function EditTitleOrDescription({
  onClose,
  accessList,
  updateAccessList,
  kind,
}: {
  onClose(): void;
  accessList: AccessListModified;
  updateAccessList(accessList: AccessList): void;
  kind: TextEditKind;
}) {
  const originalText =
    kind === 'Title' ? accessList.title : accessList.description;

  const { attempt, setAttempt } = useAttempt('');
  const [newText, setNewText] = useState(originalText);

  function handleOnCreate(validator: Validator) {
    if (!validator.validate()) {
      return;
    }

    const req: Partial<AccessList> =
      kind === 'Title' ? { title: newText } : { description: newText };

    // We don't need to setAttempt to "success"
    // since we are unmounting this after a successful
    // update.
    setAttempt({ status: 'processing' });
    accessManagementService
      .updateAccessList({
        req,
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
            <DialogTitle>Edit {kind}</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <Box>
              {kind === 'Title' ? (
                <FieldInput
                  mr={2}
                  label="Title"
                  rule={requiredField('Title is required')}
                  placeholder="title"
                  autoFocus
                  value={newText}
                  onChange={e => setNewText(e.target.value)}
                  disabled={attempt.status === 'processing'}
                />
              ) : (
                <FieldTextArea
                  label={'Description'}
                  rule={requiredMaxLength(
                    'Description must be 2048 characters or shorter.',
                    2048
                  )}
                  placeholder={'description'}
                  autoFocus
                  value={newText}
                  onChange={e => setNewText(e.target.value)}
                  disabled={attempt.status === 'processing'}
                />
              )}
            </Box>
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={attempt.status === 'processing'}
              onClick={() => handleOnCreate(validator)}
            >
              Edit {kind}
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
