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
import { requiredField } from 'shared/components/Validation/rules';

import {
  AccessList,
  accessManagementService,
} from 'e-teleport/services/accessmanagement';
import {
  CalendarDateSelect,
  ReviewDayOfMonthOption,
  ReviewFrequencyOption,
  ReviewRecurrence,
  getReviewDayOfMonthOption,
  getReviewFrequencyOption,
} from 'e-teleport/AccessListManagement/Shared/Audit';

import { AccessListModified } from '../Shared';

type Props = {
  onClose(): void;
  accessList: AccessListModified;
  updateAccessList(accessList: AccessList): void;
};

export function EditAudit({ onClose, accessList, updateAccessList }: Props) {
  const { audit } = accessList;
  const { attempt, setAttempt } = useAttempt('');
  const [auditStartDate, setAuditStartDate] = useState<Date>(audit.nextDate);
  const [frequency, setFrequency] = useState<ReviewFrequencyOption>(() =>
    getReviewFrequencyOption(audit.recurrence.frequency)
  );
  const [dayOfMonth, setDayOfMonth] = useState<ReviewDayOfMonthOption>(() =>
    getReviewDayOfMonthOption(audit.recurrence.dayOfMonth)
  );

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
            recurrence: {
              frequency: frequency.value,
              dayOfMonth: dayOfMonth.value,
            },
            nextDate: auditStartDate,
          },
        },
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
            <DialogTitle>Edit Audit</DialogTitle>
          </DialogHeader>
          <DialogContent>
            {attempt.status === 'failed' && (
              <Alert kind="danger" children={attempt.statusText} />
            )}
            <Box>
              <ReviewRecurrence
                isDisabled={attempt.status === 'processing'}
                onChangeFrequency={(o: ReviewFrequencyOption) =>
                  setFrequency(o)
                }
                onChangeDayOfMonth={(o: ReviewDayOfMonthOption) =>
                  setDayOfMonth(o)
                }
                selectedFrequency={frequency}
                selectedDayOfMonth={dayOfMonth}
              />
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
