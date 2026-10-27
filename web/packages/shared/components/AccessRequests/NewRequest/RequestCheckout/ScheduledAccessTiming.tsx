/*
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import { useState } from 'react';
import { DayPicker } from 'react-day-picker';
import 'react-day-picker/style.css';
import styled from 'styled-components';

import { Box, ButtonSecondary, Flex, LabelInput, Text } from 'design';
import { StyledDateRange } from 'design/DatePicker';
import {
  displayDateTimeWithTimeZone,
  displayShortDate,
  displayTimeWithTimeZone,
  formatTimeZoneOffset,
} from 'design/datetime';
import { Calendar as CalendarIcon } from 'design/Icon';
import { FieldSelect } from 'shared/components/FieldSelect';
import Select, { Option } from 'shared/components/Select';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import { TimeOption } from '../../Shared/types';
import { getFormattedDurationTxt } from '../../Shared/utils';
import {
  getDefaultScheduledStartTimeOption,
  getPreferredTimeOption,
  getScheduledEndTimeOptions,
  getScheduledStartLimit,
  getScheduledStartTimeOptions,
  makeAccessEndOption,
} from './utils';

export function ScheduledAccessTiming({
  start,
  maxAllowedDuration,
  durationOptions,
  selectedAccessEnd,
  onStartChange,
  onAccessEndChange,
}: {
  start: Date;
  maxAllowedDuration: number;
  durationOptions: Option<number>[];
  selectedAccessEnd: Option<number>;
  onStartChange(start?: Date): void;
  onAccessEndChange(option: Option<number>): void;
}) {
  const now = new Date();
  const startLimit = getScheduledStartLimit(now);
  const end = new Date(selectedAccessEnd.value);

  function updateStartDate(selectedDate: Date) {
    // Keep the current time when changing the date. New schedules prefer
    // 9:00 AM, and both cases fall back to the first valid time.
    const option = start
      ? getPreferredTimeOption(
          start,
          getScheduledStartTimeOptions(selectedDate)
        )
      : getDefaultScheduledStartTimeOption(selectedDate);
    if (option) {
      onStartChange(option.value);
    }
  }

  // No start time means access begins immediately after approval.
  if (!start) {
    return (
      <Box mb={1}>
        <DateControl
          label="Start Date"
          value="Immediately on Approval"
          minimum={now}
          maximum={startLimit}
          onSelect={updateStartDate}
          onImmediate={() => onStartChange(null)}
        />
        <LabelInput color="text.slightlyMuted">
          <Text mb={1}>Access Duration</Text>
          <Select
            options={durationOptions}
            onChange={onAccessEndChange}
            value={selectedAccessEnd}
          />
        </LabelInput>
        <Text typography="body3" color="text.slightlyMuted">
          Access will end on {displayDateTimeWithTimeZone(end)}
        </Text>
      </Box>
    );
  }

  function updateAccessEnd(end: Date) {
    onAccessEndChange(makeAccessEndOption(start, end));
  }

  function updateEndDate(selectedDate: Date) {
    // Keep the selected end time when changing the date, or use the first
    // valid time.
    const option = getPreferredTimeOption(
      end,
      getScheduledEndTimeOptions(selectedDate, start, maxAllowedDuration)
    );
    if (option) {
      updateAccessEnd(option.value);
    }
  }

  const maxAccessEnd = new Date(start.getTime() + maxAllowedDuration);
  const startTimeOptions = getScheduledStartTimeOptions(start, now);
  const endTimeOptions = getScheduledEndTimeOptions(
    end,
    start,
    maxAllowedDuration
  );

  return (
    <Box mb={1}>
      <TimingGrid>
        <DateControl
          label="Start Date"
          value={displayShortDate(start)}
          selected={start}
          minimum={now}
          maximum={startLimit}
          onSelect={updateStartDate}
          onImmediate={() => onStartChange(null)}
        />
        <TimeControl
          label={`Start time (${formatTimeZoneOffset(start)})`}
          value={{
            value: start,
            label: displayTimeWithTimeZone(start),
          }}
          options={startTimeOptions}
          onChange={option => onStartChange(option.value)}
        />
        <DateControl
          label="End Date"
          value={displayShortDate(end)}
          selected={end}
          minimum={start}
          maximum={maxAccessEnd}
          onSelect={updateEndDate}
        />
        <TimeControl
          label={`End time (${formatTimeZoneOffset(end)})`}
          value={{
            value: end,
            label: displayTimeWithTimeZone(end),
          }}
          options={endTimeOptions}
          onChange={option => updateAccessEnd(option.value)}
        />
      </TimingGrid>
      <LabelInput color="text.slightlyMuted">
        <Text mb={1}>Access Duration</Text>
        <Text>{getFormattedDurationTxt({ start, end })}</Text>
      </LabelInput>
    </Box>
  );
}

function DateControl({
  label,
  value,
  selected,
  minimum,
  maximum,
  onSelect,
  onImmediate,
}: {
  label: string;
  value: string;
  selected?: Date;
  minimum: Date;
  maximum: Date;
  onSelect(date: Date): void;
  onImmediate?(): void;
}) {
  const [open, setOpen] = useState(false);
  const pickerRef = useRefClickOutside<HTMLDivElement>({
    open,
    setOpen,
  });

  return (
    <Box css={{ position: 'relative' }} ref={pickerRef}>
      <LabelInput>{label}</LabelInput>
      <CalendarPicker $open={open} onClick={() => setOpen(value => !value)}>
        {value}
        <CalendarIcon ml={3} />
      </CalendarPicker>
      {open && (
        <StyledDateRange
          css={`
            position: absolute;
            z-index: 10000;
            padding: ${props => props.theme.space[1]}px;
            height: auto;
          `}
        >
          <DayPicker
            onDayClick={date => {
              onSelect(date);
              setOpen(false);
            }}
            defaultMonth={selected || minimum}
            selected={selected}
            startMonth={minimum}
            endMonth={maximum}
            disabled={{ before: minimum, after: maximum }}
            footer={
              onImmediate && (
                <Flex justifyContent="center">
                  <ButtonSecondary
                    mt={2}
                    onClick={() => {
                      onImmediate();
                      setOpen(false);
                    }}
                  >
                    Immediately
                  </ButtonSecondary>
                </Flex>
              )
            }
          />
        </StyledDateRange>
      )}
    </Box>
  );
}

function TimeControl({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: TimeOption;
  options: TimeOption[];
  onChange(option: TimeOption): void;
}) {
  return (
    <Box>
      <LabelInput>{label}</LabelInput>
      <FieldSelect mb={0} value={value} options={options} onChange={onChange} />
    </Box>
  );
}

const TimingGrid = styled.div`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: ${props => props.theme.space[2]}px;
  margin-bottom: ${props => props.theme.space[2]}px;
`;

const CalendarPicker = styled(Flex)<{ $open: boolean }>`
  height: 40px;
  border: 1px solid ${props => props.theme.colors.interactive.tonal.neutral[2]};
  border-radius: ${props => props.theme.radii[2]}px;
  padding: 0 ${props => props.theme.space[2]}px 0
    ${props => props.theme.space[3]}px;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;

  &:hover {
    border-color: ${props => props.theme.colors.text.muted};
  }

  ${props =>
    props.$open &&
    `border-color: ${props.theme.colors.interactive.solid.primary.default};`}
`;
