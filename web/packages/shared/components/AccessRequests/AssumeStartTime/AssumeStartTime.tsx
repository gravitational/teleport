/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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
import styled from 'styled-components';

import 'react-day-picker/dist/style.css';

import { addMonths } from 'date-fns';

import { Box, ButtonIcon, Flex, LabelInput } from 'design';
import { ButtonSecondary } from 'design/Button';
import { displayShortDate } from 'design/datetime';
import { Calendar as CalendarIcon, Refresh as RefreshIcon } from 'design/Icon';
import FieldSelect from 'shared/components/FieldSelect';
import Validation from 'shared/components/Validation';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';
import { AccessRequest } from 'shared/services/accessRequests';

import { StyledDateRange } from 'teleport/components/DayPicker/Shared';

import { TimeOption } from '../Shared/types';
import {
  convertStartToTimeOption,
  getMaxAssumableDate,
  getTimeOptions,
} from './timeOptions';

export function AssumeStartTime({
  start,
  onStartChange,
  accessRequest,
  reviewing = false,
}: {
  start: Date;
  onStartChange(s?: Date): void;
  accessRequest: AccessRequest;
  reviewing?: boolean;
}) {
  const [wantImmediate, setWantImmediate] = useState(
    () => !accessRequest.assumeStartTime
  );

  const [showDayPicker, setShowDayPicker] = useState(false);
  const dayPickerRef = useRefClickOutside<HTMLDivElement>({
    open: showDayPicker,
    setOpen: setShowDayPicker,
  });

  function startImmediately() {
    setWantImmediate(true);
    // Overwrite the requested start time
    // with now time.
    if (accessRequest.assumeStartTime) {
      onStartChange(new Date());
    } else {
      // This case means the request was already
      // requesting to gain access immediately so
      // nothing to overwrite here.
      onStartChange(null);
    }

    setShowDayPicker(false);
  }

  // Updates the start "date" part of a Date, and we pre-select option that is
  // closest to one week for the selected date. On every update, it re-calculates
  // the time options and duration options available for the selected date.
  function updateStartDate(selectedDate: Date) {
    setWantImmediate(false);
    const updatedTimesOptions = getTimeOptions(
      selectedDate,
      accessRequest,
      reviewing
    );

    if (!updatedTimesOptions.length) {
      // There is no other time options for the current duration.
      setShowDayPicker(false);
      return;
    }

    onStartChange(updatedTimesOptions[0].value);
    setShowDayPicker(false);
  }

  // Updates the start "time" part of a Date. On every update, it re-calculates
  // the duration options available for the selected time.
  function updateStartTime(time: TimeOption) {
    setWantImmediate(false);
    onStartChange(time?.value);
  }

  let startDate = accessRequest.created;
  if (reviewing) {
    startDate = new Date();
  }

  let startDateText = 'Immediately';
  let startTime: TimeOption;
  let startTimeOptions: TimeOption[] = [];

  const startOrRequestedDate = start || accessRequest.assumeStartTime;
  if (!wantImmediate && startOrRequestedDate) {
    startDateText = displayShortDate(startOrRequestedDate);
    startTime = convertStartToTimeOption(
      startOrRequestedDate,
      !start && !!accessRequest.assumeStartTime
    );
    startTimeOptions = getTimeOptions(
      startOrRequestedDate,
      accessRequest,
      reviewing
    );
  }

  // This flag is used to give reviewer the ability to reset start date/time
  // to the originally requested date/time.
  const showResetDateTime = reviewing && start;

  return (
    <Validation>
      <Flex gap={2} alignItems="end" mb={2}>
        <Box css={{ position: 'relative' }} ref={dayPickerRef}>
          <LabelInput>Start Date</LabelInput>
          <CalendarPicker
            onClick={() => {
              setShowDayPicker(s => !s);
            }}
            maxWidth="270px"
            minWidth="200px"
          >
            {startDateText}
            <CalendarIcon ml={3} />
          </CalendarPicker>
          {showDayPicker && (
            <StyledDateRange
              css={`
                position: absolute;
                z-index: 10000;
                padding: ${p => p.theme.space[1]}px;
                height: auto;
                .rdp {
                  --rdp-cell-size: 30px; /* Size of the day cells. */
                  --rdp-caption-font-size: 14px; /* Font size for the caption labels. */
                }
              `}
            >
              <DayPicker
                data-testid="day-picker"
                onDayClick={updateStartDate}
                defaultMonth={startDate}
                selected={startOrRequestedDate}
                fromMonth={startDate}
                // Incase part of 7 days falls to the next month.
                // Allows user to select day from next month
                // and disables navigating rest of month.
                toMonth={addMonths(startDate, 1)}
                // Disables before today, and after 7th day.
                disabled={[
                  {
                    before: startDate,
                    after: getMaxAssumableDate(accessRequest),
                  },
                ]}
                footer={
                  <Flex css={{ justifyContent: 'center' }}>
                    <ButtonSecondary
                      mt={2}
                      onClick={startImmediately}
                      textTransform="none"
                    >
                      Immediately
                    </ButtonSecondary>
                  </Flex>
                }
              />
            </StyledDateRange>
          )}
        </Box>
        {startTime && (
          <Box>
            <LabelInput>Start Time</LabelInput>
            <FieldSelect
              mb={0}
              width="190px"
              isSearchable={true}
              options={startTimeOptions}
              value={startTime}
              onChange={updateStartTime}
            />
          </Box>
        )}
        {showResetDateTime && (
          <ButtonIcon
            data-testid="reset-btn"
            onClick={() => updateStartTime(null)}
            title="Reset to requested time"
            mb={1}
          >
            <RefreshIcon size="medium" />
          </ButtonIcon>
        )}
      </Flex>
    </Validation>
  );
}

const CalendarPicker = styled(Flex)`
  height: 40px;
  border: 1px solid ${p => p.theme.colors.text.muted};
  border-radius: ${p => p.theme.radii[2]}px;
  padding: 0 ${p => p.theme.space[2]}px;
  align-items: center;
  justify-content: space-between;
  cursor: pointer;
  &:hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
    border: 1px solid ${p => p.theme.colors.text.slightlyMuted};
  }
`;
