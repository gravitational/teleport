/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import 'react-day-picker/dist/style.css';

import styled from 'styled-components';

import { Box, ButtonBorder, Text } from 'design';
import { displayDate } from 'design/datetime';
import Dialog from 'design/DialogConfirmation';
import { Calendar } from 'design/Icon';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import { CustomRange } from './Custom';
import { EventRange } from './utils';

export default function DateRange({ ml, range, onChangeRange }: Props) {
  const [isPickerOpen, openDayPicker] = useState(false);

  const dayPickerRef = useRefClickOutside<HTMLDivElement>({
    open: isPickerOpen,
    setOpen: openDayPicker,
  });

  function onClosePicker() {
    openDayPicker(false);
  }

  function onSetCustomRange(from: Date, to: Date) {
    onChangeRange({ isCustom: true, from, to });
    onClosePicker();
  }

  return (
    <>
      <Box ml={ml}>
        <DateRangeButton onClick={() => openDayPicker(true)}>
          <Calendar size={16} mr={2} />
          {range ? (
            <Text>{`${displayDate(range.from)} - ${displayDate(range.to)}`}</Text>
          ) : (
            <Text color="text.muted">Select date range...</Text>
          )}
        </DateRangeButton>
      </Box>
      <Dialog
        dialogCss={() => ({ padding: '0' })}
        disableEscapeKeyDown={false}
        onClose={onClosePicker}
        open={isPickerOpen}
      >
        <CustomRange
          initialRange={
            range
              ? { from: range.from, to: range.to }
              : { from: new Date(), to: new Date() }
          }
          onChange={onSetCustomRange}
          ref={dayPickerRef}
        />
      </Dialog>
    </>
  );
}

const DateRangeButton = styled(ButtonBorder)`
  display: flex;
  align-items: center;
  justify-content: flex-start;
  min-width: 240px;
  padding: 8px 12px;
`;

type Props = {
  ml?: string | number;
  range: EventRange | undefined;
  onChangeRange: (range: EventRange) => void;
};
