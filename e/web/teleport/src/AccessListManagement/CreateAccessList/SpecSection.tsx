import { Box } from 'design';
import FieldInput from 'shared/components/FieldInput';
import { requiredField } from 'shared/components/Validation/rules';

import {
  CalendarDateSelect,
  ReviewDayOfMonthOption,
  ReviewFrequencyOption,
  ReviewRecurrence,
} from '../Shared/Audit';
import { useCreateAccessList } from './CreateAccessListContextProvider';

export const SpecSection = () => {
  const { spec, setSpec, createAttempt } = useCreateAccessList();
  const isDisabled = createAttempt.status === 'processing';

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
