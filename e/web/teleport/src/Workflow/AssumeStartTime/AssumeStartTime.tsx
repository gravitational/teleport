import { useState, useEffect } from 'react';
import styled from 'styled-components';
import { DayPicker } from 'react-day-picker';
import 'react-day-picker/dist/style.css';
import { addMonths, format } from 'date-fns';
import FieldSelect from 'shared/components/FieldSelect';
import Validation from 'shared/components/Validation';
import { Flex, Box, LabelInput, Text } from 'design';
import { Calendar as CalendarIcon } from 'design/Icon';
import Select, { Option } from 'shared/components/Select';
import { StyledDateRange } from 'teleport/components/DayPicker/Shared';
import { ButtonSecondary } from 'design/Button';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';
import { ToolTipInfo } from 'shared/components/ToolTip';

import { AccessRequest } from 'e-teleport/services/workflow';

import { Start, TimeOption } from '../Shared/types';
import { getStartDateTime } from '../Shared/utils';

import {
  getDurationOptionIndexClosestToOneWeek,
  getDurationOptionsFromStartTime,
  getTimeOptions,
} from './utils';

export function AssumeStartTime({
  start,
  setStart,
  accessRequest,
  maxDuration,
  setMaxDuration,
}: {
  start: Start;
  setStart(s?: Start): void;
  accessRequest: AccessRequest;
  maxDuration: Option<number>;
  setMaxDuration(s: Option<number>): void;
}) {
  // Options for extending or shortening the access request duration.
  const [durationOptions, setDurationOptions] = useState<Option<number>[]>([]);
  // Options for selecting a custom start time.
  const [startTimeOptions, setStartTimeOptions] = useState<TimeOption[]>([]);

  const [showDayPicker, setShowDayPicker] = useState(false);
  const dayPickerRef = useRefClickOutside<HTMLDivElement>({
    open: showDayPicker,
    setOpen: setShowDayPicker,
  });

  useEffect(() => {
    defaultStartAndDuration();
  }, []);

  function defaultStartAndDuration() {
    setStart(undefined);
    setShowDayPicker(false);

    const created = accessRequest.created;
    const options = getDurationOptionsFromStartTime(
      created,
      {
        value: {
          minutes: created.getMinutes(),
          militaryHrs: created.getHours(),
        },
        label: '', // unused
      },
      accessRequest
    );

    setDurationOptions(options);
    if (options.length > 0) {
      const durationIndex = getDurationOptionIndexClosestToOneWeek(
        options,
        accessRequest.created
      );
      setMaxDuration(options[durationIndex]);
    }
  }

  function updateAccessDuration(selectedDate: Date, selectedTime: TimeOption) {
    const updatedDurationOpts = getDurationOptionsFromStartTime(
      selectedDate,
      selectedTime,
      accessRequest
    );

    const durationIndex = getDurationOptionIndexClosestToOneWeek(
      updatedDurationOpts,
      selectedDate
    );

    setMaxDuration(updatedDurationOpts[durationIndex]);
    setDurationOptions(updatedDurationOpts);
  }

  // Updates the start "date" part of a Date, and we pre-select option that is
  // closest to one week for the selected date. On every update, it re-calculates
  // the time options and duration options available for the selected date.
  function updateStartDate(selectedDate: Date) {
    const updatedTimesOptions = getTimeOptions(selectedDate, accessRequest);
    setStartTimeOptions(updatedTimesOptions);

    if (!updatedTimesOptions.length) {
      // There is no other time options for the current duration.
      setShowDayPicker(false);
      return;
    }

    updateAccessDuration(selectedDate, updatedTimesOptions[0]);
    setStart({ ...start, date: selectedDate, time: updatedTimesOptions[0] });
    setShowDayPicker(false);
  }

  // Updates the start "time" part of a Date. On every update, it re-calculates
  // the duration options available for the selected time.
  function updateStartTime(time: TimeOption) {
    const startDate = getStartDateTime({ ...start, time });

    updateAccessDuration(startDate, time);
    setStart({ ...start, time });
  }

  const startDate = accessRequest.created;

  let startDateText = 'Immediately';
  if (start?.date) {
    startDateText = format(start.date, 'LLLL dd, yyyy');
  }

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
            minWidth="180px"
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
                selected={start?.date}
                fromMonth={startDate}
                // Incase part of 7 days falls to the next month.
                // Allows user to select day from next month
                // and disables navigating rest of month.
                toMonth={addMonths(startDate, 1)}
                // Disables before today, and after 7th day.
                disabled={[
                  {
                    after:
                      accessRequest.maxDuration.getHours() - 1 > 0
                        ? accessRequest.maxDuration
                        : accessRequest.created,
                    before: startDate,
                  },
                ]}
                footer={
                  <Flex css={{ justifyContent: 'center' }}>
                    <ButtonSecondary
                      mt={2}
                      onClick={defaultStartAndDuration}
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
        {start?.time && (
          <Box>
            <LabelInput>Start Time</LabelInput>
            <FieldSelect
              mb={0}
              width="190px"
              isSearchable={true}
              options={startTimeOptions}
              value={start?.time}
              onChange={updateStartTime}
            />
          </Box>
        )}
      </Flex>
      <LabelInput typography="body2" color="text.slightlyMuted">
        <Flex alignItems="center">
          <Text mr={1}>Access Duration</Text>
          <ToolTipInfo>
            How long the access should be granted for. Note that the time it
            takes to approve this request is subtracted from the duration you
            select.
          </ToolTipInfo>
        </Flex>

        <Select
          options={durationOptions}
          onChange={(option: Option<number>) => {
            setMaxDuration(option);
          }}
          value={maxDuration}
        />
      </LabelInput>
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
  :hover {
    background-color: ${p => p.theme.colors.spotBackground[0]};
    border: 1px solid ${p => p.theme.colors.text.slightlyMuted};
  }
`;
