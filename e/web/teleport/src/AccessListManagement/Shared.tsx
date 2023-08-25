import React, { useState, PropsWithChildren } from 'react';
import styled from 'styled-components';
import { format } from 'date-fns';
import { Popover, Text, Box, LabelInput, Flex } from 'design';
import { Calendar as CalendarIcon } from 'design/Icon';
import { StyledSelect, Option } from 'shared/components/Select';
import { User } from 'teleport/services/user';
import { Resource, KindRole } from 'teleport/services/resources';
import { useRule } from 'shared/components/Validation';

import { DatePicker } from './DatePicker';

export const dateFormat = 'MM/dd/yyyy';
export type UserOption = Option<User>;
export type RoleOption = Option<Resource<KindRole>>;
export type UserKind = 'Member' | 'Owner';

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
        <Box>{date ? format(date, dateFormat) : 'Select a Date'}</Box>
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
