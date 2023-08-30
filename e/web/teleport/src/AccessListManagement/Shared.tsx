import React, { useState, PropsWithChildren } from 'react';
import styled from 'styled-components';
import { format } from 'date-fns';
import { Popover, Text, Box, LabelInput, Flex, Label } from 'design';
import { Calendar as CalendarIcon } from 'design/Icon';
import { StyledSelect, Option } from 'shared/components/Select';
import { User } from 'teleport/services/user';
import { useRule } from 'shared/components/Validation';

import { DatePicker } from './DatePicker';

export type UserOption = Option<User>;
export type EditKind = 'Member' | 'Owner' | 'Grants';

interface RuleResult {
  valid: boolean;
  message?: string;
}
type RuleFunc<T> = (v: T) => () => RuleResult;
const noopRule: RuleFunc<any> = () => () => ({ valid: true });

// TODO(lisa): would benefit moving it to shared package
// similar to FieldInput and FieldSelect
export function FieldSelectAndCreatableWrapper<T>({
  label,
  value,
  rule = noopRule,
  children,
}: PropsWithChildren<{
  label: string;
  value: T[];
  rule?: RuleFunc<T[]>;
}>) {
  const { valid, message } = useRule(rule(value));
  const hasError = Boolean(!valid);
  const labelText = hasError ? message : label;
  return (
    <Box mb={4}>
      {label && (
        <LabelInput htmlFor={'select'} hasError={hasError}>
          {labelText}
        </LabelInput>
      )}
      <StyledSelect hasError={hasError}>{children}</StyledSelect>
    </Box>
  );
}

// TODO(lisa): move this to 'shared/ToolTip' package
// and refactor ToolTipInfo with this.
export const ToolTipText: React.FC<{
  tipContent: React.ReactElement;
  fontSize?: number;
}> = ({ tipContent, fontSize = 10, children }) => {
  const [anchorEl, setAnchorEl] = useState();
  const open = Boolean(anchorEl);

  function handlePopoverOpen(event) {
    setAnchorEl(event.currentTarget);
  }

  function handlePopoverClose() {
    setAnchorEl(null);
  }

  return (
    <>
      <span
        aria-owns={open ? 'mouse-over-popover' : undefined}
        onMouseEnter={handlePopoverOpen}
        onMouseLeave={handlePopoverClose}
      >
        {children}
      </span>
      <Popover
        modalCss={modalCss}
        onClose={handlePopoverClose}
        open={open}
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'left',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'left',
        }}
      >
        <StyledOnHover px={2} py={1} fontSize={`${fontSize}px`}>
          {tipContent}
        </StyledOnHover>
      </Popover>
    </>
  );
};

const modalCss = () => `
  pointer-events: none;
`;

const StyledOnHover = styled(Text)`
  color: ${props => props.theme.colors.text.main};
  background-color: ${props => props.theme.colors.tooltip.background};
  max-width: 350px;
`;

export const TruncatingLabel = styled(Label)`
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  max-width: 160px;
`;

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
        alignItems="center"
        justifyContent="space-between"
        px={2}
        borderRadius={2}
        dateSelected={Boolean(date)}
      >
        <Box>{validDate ? format(date, dateFormat) : 'Select a Date'}</Box>
        <CalendarIcon />
      </CalendarInput>
      {showDatePicker && (
        <DatePicker
          selectedDate={date}
          onClose={() => setShowDatePicker(false)}
          onPickDate={newDate => {
            onChange(newDate);
            setShowDatePicker(false);
          }}
        />
      )}
    </>
  );
};

const CalendarInput = styled(Flex)`
  color: ${p => (p.dateSelected ? 'inherit' : p.theme.colors.text.disabled)};
  height: 40px;
  border: 1px solid ${p => p.theme.colors.text.muted};
  cursor: pointer;
  :hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
    border: 1px solid ${p => p.theme.colors.text.slightlyMuted};
  }

  ${({ hasError, theme }) => {
    if (hasError) {
      return {
        border: `2px solid ${theme.colors.error.main}`,
      };
    }
  }}
`;

const HOURS_PER_MONTH = 730;
const SECONDS_PER_MONTH = 2628000; // approx. 0.9999989041 month (google)
const SECONDS_PER_DAY = 86400;

type OptionWithKey = Option & { key: number };
export const auditFrequencyOpts: OptionWithKey[] = [
  { label: 'every 1 month', value: `${1 * HOURS_PER_MONTH}h`, key: 1 },
  { label: 'every 3 months', value: `${3 * HOURS_PER_MONTH}h`, key: 3 },
  { label: 'every 6 months', value: `${6 * HOURS_PER_MONTH}h`, key: 6 },
];

// calculateMonthsDaysFromDuration is an approximate calculation
// of months and days from seconds.
export function calculateMonthsDaysFromDuration(duration: string) {
  // Assuming the backend will always return the format string '0h0m0s'
  const [hrs = 0, mins = 0, secs = 0] = duration.split(/h|m|s/);
  let totalSeconds = Number(hrs) * 3600 + Number(mins) * 60 + Number(secs);

  // This is an approximate value.
  const months = Math.floor(totalSeconds / SECONDS_PER_MONTH);
  totalSeconds %= SECONDS_PER_MONTH;
  const days = Math.floor(totalSeconds / SECONDS_PER_DAY);
  // Drop the rest.
  // We are only handling days, despite the UI having hard coded
  // frequency limits (see auditFrequencyOpts) just in case a user
  // tries to create an access list through the CLI, which allows
  // you to fine tune the frequency down the seconds.

  return { months, days };
}

const dateFormat = 'MM/dd/yyyy';
export function getFormattedDate(d: Date) {
  if (!d || isNaN(d.getTime())) {
    return '';
  }

  // The zero value for golang Date comes back as "0001-01-01T00:00:00Z"
  // which is January 1, year 1.
  // JS zero date is January 1, 1970  which is "greater" than
  // golang's zero value. So it's safe to assume that backend Dates
  // that are less than JS's zero value means the date was not set.
  const zeroDate = new Date(0);
  const thisDate = new Date(d);
  if (thisDate <= zeroDate) {
    return '';
  }

  return format(thisDate, dateFormat);
}
