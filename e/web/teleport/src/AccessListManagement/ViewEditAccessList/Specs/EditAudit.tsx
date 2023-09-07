import React, { useState } from 'react';
import { ButtonPrimary, ButtonSecondary, Alert, Box } from 'design';

import useAttempt from 'shared/hooks/useAttemptNext';
import Dialog, {
  DialogHeader,
  DialogTitle,
  DialogContent,
  DialogFooter,
} from 'design/Dialog';
import Validation, { Validator } from 'shared/components/Validation';
import { Option } from 'shared/components/Select';
import { requiredField } from 'shared/components/Validation/rules';

import { accessManagementService } from 'e-teleport/services/accessmanagement';
import { AuditReviewFrequencySelectField } from 'e-teleport/AccessListManagement/CreateAccessList/SpecSection';
import {
  CalendarDateSelect,
  auditFrequencyOpts,
  calculateMonthsDaysFromDuration,
} from 'e-teleport/AccessListManagement/Shared';

import { AccessListModified } from '../ViewEditAccessList';

type Props = {
  onClose(): void;
  accessList: AccessListModified;
  fetchAccessList(): Promise<void | boolean>;
};

export function EditAudit({ onClose, accessList, fetchAccessList }: Props) {
  const { audit } = accessList;
  const { attempt, setAttempt } = useAttempt('');
  const [frequency, setFrequency] = useState<Option>(() => {
    const { months } = calculateMonthsDaysFromDuration(audit.frequency);

    // Leave field empty if stored frequency doesn't match the
    // hard coded ones on UI.
    return auditFrequencyOpts.find(o => o.key === months);
  });
  const [auditStartDate, setAuditStartDate] = useState<Date>(audit.nextDate);

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
        req: {
          audit: {
            frequency: frequency.value,
            nextDate: auditStartDate,
          },
        },
        original: accessList,
      })
      .then(() => {
        onClose();
        fetchAccessList();
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
            <DialogTitle>Edit Audit</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <Box width="50%">
              <AuditReviewFrequencySelectField
                isDisabled={attempt.status === 'processing'}
                onChangeFrequency={(o: Option) => setFrequency(o)}
                selectedFrequency={frequency}
              />
            </Box>
            <Box width="50%" mb={3}>
              <CalendarDateSelect
                date={auditStartDate}
                onChange={(newDate: Date) => setAuditStartDate(newDate)}
                rule={requiredField('Audit date required')}
                label="Audit Date"
              />
            </Box>
          </DialogContent>
          <DialogFooter>
            <ButtonPrimary
              mr="3"
              disabled={attempt.status === 'processing'}
              onClick={() => handleOnCreate(validator)}
            >
              Edit Audit
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
