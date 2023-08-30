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

import {
  AccessListAudit,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import { AuditReviewFrequencySelectField } from 'e-teleport/AccessListManagement/CreateAccessList/SpecSection';
import {
  CalendarDateSelect,
  auditFrequencyOpts,
  calculateMonthsDaysFromDuration,
} from 'e-teleport/AccessListManagement/Shared';

type Props = {
  onClose(): void;
  audit: AccessListAudit;
  fetchAccessList(): Promise<void | boolean>;
};

export function EditAudit({ onClose, audit, fetchAccessList }: Props) {
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
        audit: {
          frequency: frequency.value,
          nextDate: auditStartDate,
        },
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
