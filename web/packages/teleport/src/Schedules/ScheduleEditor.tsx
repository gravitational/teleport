/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { Dispatch, SetStateAction } from 'react';
import { StylesConfig } from 'react-select';
import styled, { useTheme } from 'styled-components';

import { Box, ButtonPrimary, Flex, Text } from 'design';
import { LabelContent } from 'design/LabelInput/LabelInput';
import Select, { Option, SelectCreatable } from 'shared/components/Select';
import { useRule } from 'shared/components/Validation';

import { TimeOptions, TimezoneOptions, WeekdayOptions } from './const';
import { validSchedule, validShift } from './rules';
import { NewShift, Schedule, Shift, Weekday } from './types';

export const ScheduleEditor = ({
  schedule,
  setSchedule,
}: {
  schedule: Schedule;
  setSchedule: Dispatch<SetStateAction<Schedule>>;
}) => {
  const theme = useTheme();
  const { valid, message } = useRule(validSchedule(schedule));

  const setTimezone = (option: Option) => {
    setSchedule(prev => ({
      ...prev,
      timezone: option,
    }));
  };

  const toggleWeekday = (weekday: Weekday) => {
    setSchedule(prev => {
      const { [weekday]: exists, ...shifts } = prev.shifts;
      return {
        ...prev,
        shifts: exists
          ? shifts
          : {
              ...prev.shifts,
              [weekday]: NewShift(),
            },
      };
    });
  };

  const setShift = (weekday: string, shift: Shift) => {
    setSchedule(prev => ({
      ...prev,
      shifts: {
        ...prev.shifts,
        [weekday]: shift,
      },
    }));
  };

  return (
    <Flex flexDirection="column" width="330px" gap={3}>
      <Box>
        <LabelContent>Time Zone</LabelContent>
        <Select
          value={schedule.timezone}
          onChange={option => setTimezone(option)}
          options={TimezoneOptions}
        />
      </Box>
      <Flex gap={2}>
        {WeekdayOptions.map(weekday => (
          <ButtonPrimary
            key={weekday.value}
            size="large"
            width={40}
            inputAlignment={true}
            intent={schedule.shifts?.[weekday.value] ? 'primary' : 'neutral'}
            onClick={() => toggleWeekday(weekday.value)}
          >
            {weekday.label}
          </ButtonPrimary>
        ))}
      </Flex>
      <Box>
        <WeekdayScheduleTable>
          <tbody>
            {!!schedule.shifts &&
              WeekdayOptions.filter(
                weekday => !!schedule.shifts[weekday.value]
              ).map(weekday => {
                return (
                  <tr key={weekday.value}>
                    <td>
                      <Text>{weekday.value}</Text>
                    </td>
                    <td colSpan={3}>
                      <ShiftSelect
                        shift={schedule.shifts[weekday.value]}
                        setShift={shift => setShift(weekday.value, shift)}
                      />
                    </td>
                  </tr>
                );
              })}
          </tbody>
        </WeekdayScheduleTable>
        {!valid && (
          <Flex>
            <Text color={theme.colors.interactive.solid.danger.default}>
              {message}
            </Text>
          </Flex>
        )}
      </Box>
    </Flex>
  );
};

const ShiftSelect = ({
  shift,
  setShift,
}: {
  shift: Shift;
  setShift: (option: Shift) => void;
}) => {
  const theme = useTheme();
  const { valid } = useRule(validShift(shift));

  return (
    <StyledFlex gap={3} hasError={!valid}>
      <Box flex="1" textAlign="center">
        <SelectCreatable
          value={shift.startTime}
          onChange={option => setShift({ ...shift, startTime: option })}
          options={TimeOptions}
          components={{ DropdownIndicator: () => null }}
          stylesConfig={selectCreatableStyles}
        />
      </Box>
      <Text color={theme.colors.text.muted}>to</Text>
      <Box flex="1" textAlign="center">
        <SelectCreatable
          value={shift.endTime}
          onChange={option => setShift({ ...shift, endTime: option })}
          options={TimeOptions}
          components={{ DropdownIndicator: () => null }}
          stylesConfig={selectCreatableStyles}
        />
      </Box>
    </StyledFlex>
  );
};

const WeekdayScheduleTable = styled.table`
  width: 100%;
  border-collapse: collapse;
  /*
   * Using fixed layout seems to be the only way to prevent the internal input
   * padding from somehow influencing the column width. As the padding is
   * variable (and reflects the error state), we'd rather avoid column width
   * changes while editing.
   */
  table-layout: fixed;

  & td {
    padding: 0;
    padding-bottom: ${props => props.theme.space[2]}px;
  }
`;

const StyledFlex = styled(Flex)<{
  hasError?: boolean;
}>`
  align-items: center;
  text-align: center;

  border: 1px solid;
  border-radius: 4px;
  border-color: ${({ hasError, theme }) =>
    hasError
      ? theme.colors.interactive.solid.danger.default
      : theme.colors.interactive.tonal.neutral[2]};
  &:hover {
    border-color: ${({ hasError, theme }) =>
      hasError
        ? theme.colors.interactive.solid.danger.default
        : theme.colors.interactive.solid.primary.default};
  }
`;

const selectCreatableStyles: StylesConfig = {
  control: base => ({
    ...base,
    border: 'none !important',
  }),
  valueContainer: base => ({
    ...base,
    justifyContent: 'center',
    padding: '0px !important',
  }),
  input: base => ({
    ...base,
    overflow: 'hidden',
  }),
};
