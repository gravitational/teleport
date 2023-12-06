import React from 'react';
import { Box } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';

import {
  CalendarDateSelect,
  ReviewDayOfMonthOption,
  ReviewFrequencyOption,
  ReviewRecurrence,
} from '../Shared/Audit';

type Props = {
  spec: Spec;
  setSpec(s: Spec): void;
  isDisabled: boolean;
};

export type Spec = {
  title: string;
  description: string;
  reviewDayOfMonth: ReviewDayOfMonthOption;
  reviewFrequency: ReviewFrequencyOption;
  auditStartDate: Date;
};

export const SpecSection = ({ spec, setSpec, isDisabled }: Props) => {
  return (
    <>
      <FieldInput
        label="Title"
        rule={requiredField('Title is required')}
        placeholder="Tile"
        autoFocus={!isDisabled}
        value={spec.title}
        onChange={e => setSpec({ ...spec, title: e.target.value })}
      />
      <FieldInput
        label="Description (Optional)"
        placeholder="Description"
        value={spec.description}
        onChange={e => setSpec({ ...spec, description: e.target.value })}
      />
      <Box>
        <ReviewRecurrence
          isDisabled={isDisabled}
          onChangeFrequency={(o: ReviewFrequencyOption) =>
            setSpec({ ...spec, reviewFrequency: o })
          }
          onChangeDayOfMonth={(o: ReviewDayOfMonthOption) =>
            setSpec({ ...spec, reviewDayOfMonth: o })
          }
          selectedFrequency={spec.reviewFrequency}
          selectedDayOfMonth={spec.reviewDayOfMonth}
        />
        <CalendarDateSelect
          date={spec.auditStartDate}
          onChange={(newDate: Date) =>
            setSpec({ ...spec, auditStartDate: newDate })
          }
          rule={requiredField('Review deadline required')}
          label="Deadline for First Review"
        />
      </Box>
    </>
  );
};
