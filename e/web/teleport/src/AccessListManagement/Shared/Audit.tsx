import { useState } from 'react';
import styled from 'styled-components';
import { Flex, LabelInput, Box } from 'design';
import { Calendar as CalendarIcon } from 'design/Icon';
import FieldSelect from 'shared/components/FieldSelect';
import { Option } from 'shared/components/Select';
import { useRule } from 'shared/components/Validation';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';
import { displayDate } from 'design/datetime';

import {
  ReviewDayOfMonth,
  ReviewFrequency,
} from 'e-teleport/services/accessmanagement';

import { DatePicker } from '../DatePicker';

import { RuleFunc, noopRule } from './rules';

export type ReviewFrequencyOption = Option<ReviewFrequency>;

export const reviewFrequencyOpts: ReviewFrequencyOption[] = [
  { label: 'every month', value: ReviewFrequency.OneMonth },
  { label: 'every 3 months', value: ReviewFrequency.ThreeMonths },
  { label: 'every 6 months', value: ReviewFrequency.SixMonths },
  { label: 'every 12 months', value: ReviewFrequency.OneYear },
];

export function getReviewFrequencyOption(
  frequency: ReviewFrequency
): ReviewFrequencyOption {
  switch (frequency) {
    case ReviewFrequency.OneMonth:
      return reviewFrequencyOpts[0];
    case ReviewFrequency.ThreeMonths:
      return reviewFrequencyOpts[1];
    case ReviewFrequency.OneYear:
      return reviewFrequencyOpts[3];
    default:
      return reviewFrequencyOpts[2]; // 6 months
  }
}

export type ReviewDayOfMonthOption = Option<ReviewDayOfMonth>;

export const reviewDayOfMonthOpts: ReviewDayOfMonthOption[] = [
  { label: 'first day of month', value: ReviewDayOfMonth.FirstDayOfMonth },
  {
    label: 'fifteenth day of month',
    value: ReviewDayOfMonth.FifteenthDayOfMonth,
  },
  { label: 'last day of month', value: ReviewDayOfMonth.LastDayOfMonth },
];

export function getReviewDayOfMonthOption(
  dayOfMonth: ReviewDayOfMonth
): ReviewDayOfMonthOption {
  switch (dayOfMonth) {
    case ReviewDayOfMonth.FifteenthDayOfMonth:
      return reviewDayOfMonthOpts[1];
    case ReviewDayOfMonth.LastDayOfMonth:
      return reviewDayOfMonthOpts[2];
    default:
      return reviewDayOfMonthOpts[0]; // first day of month
  }
}

export const ReviewRecurrence = ({
  isDisabled,
  onChangeFrequency,
  onChangeDayOfMonth,
  selectedFrequency,
  selectedDayOfMonth,
}: {
  isDisabled: boolean;
  onChangeFrequency(o: ReviewFrequencyOption): void;
  onChangeDayOfMonth(o: ReviewDayOfMonthOption): void;
  selectedFrequency: ReviewFrequencyOption;
  selectedDayOfMonth: ReviewDayOfMonthOption;
}) => {
  return (
    <Flex gap={3}>
      <FieldSelect
        width="50%"
        label="Review Frequency"
        isSearchable={true}
        options={reviewFrequencyOpts}
        isDisabled={isDisabled}
        onChange={onChangeFrequency}
        value={selectedFrequency}
        menuPosition="fixed"
      />
      <FieldSelect
        width="50%"
        label="Review Day Of Month"
        isSearchable={true}
        options={reviewDayOfMonthOpts}
        isDisabled={isDisabled}
        onChange={onChangeDayOfMonth}
        value={selectedDayOfMonth}
        menuPosition="fixed"
      />
    </Flex>
  );
};

// TODO(lisa): would benefit moving it to shared package
// similar to FieldInput and FieldSelect
export const CalendarDateSelect = ({
  date,
  onChange,
  rule = noopRule,
  label,
}: {
  onChange(date: Date): void;
  date: Date;
  rule?: RuleFunc<string>;
  label: string;
}) => {
  const [showDatePicker, setShowDatePicker] = useState(false);
  const dayPickerRef = useRefClickOutside<HTMLDivElement>({
    open: showDatePicker,
    setOpen: setShowDatePicker,
  });

  const { valid, message } = useRule(rule(date ? date.toDateString() : ''));
  const hasError = Boolean(!valid);
  const labelText = hasError ? message : label;

  const validDate = date && !isNaN(date.getTime());

  return (
    <>
      <LabelInput hasError={hasError}>{labelText}</LabelInput>
      <CalendarInput
        hasError={hasError}
        onClick={() => setShowDatePicker(true)}
        onKeyUp={e => {
          if (e.key === 'Enter') {
            setShowDatePicker(!showDatePicker);
          }
        }}
        alignItems="center"
        justifyContent="space-between"
        px={2}
        borderRadius={2}
        dateSelected={Boolean(date)}
        tabIndex={0}
      >
        <Box>{validDate ? displayDate(date) : 'Select a Date'}</Box>
        <CalendarIcon />
      </CalendarInput>
      {showDatePicker && (
        <DatePicker
          ref={dayPickerRef}
          selectedDate={date}
          onPickDate={newDate => {
            onChange(newDate);
            setShowDatePicker(false);
          }}
        />
      )}
    </>
  );
};

// TODO(kiosion): Should be an HTMLButtonElement instead of a div,
// this is not currently accessible.
const CalendarInput = styled(Flex)<{
  dateSelected?: boolean;
  hasError?: boolean;
}>`
  color: ${p => (p.dateSelected ? 'inherit' : p.theme.colors.text.disabled)};
  height: 40px;
  border: 1px solid ${p => p.theme.colors.text.muted};
  cursor: pointer;

  &:hover,
  &:focus-visible {
    background-color: ${p => p.theme.colors.spotBackground[0]};
    border: 1px solid ${p => p.theme.colors.text.slightlyMuted};
  }

  &:focus-visible {
    border: 1px solid ${p => p.theme.colors.brand};
    outline: none;
  }

  ${({ hasError, theme }) => {
    if (hasError) {
      return {
        border: `2px solid ${theme.colors.error.main}`,
      };
    }
  }}
`;
