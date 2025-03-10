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
import { components, ValueContainerProps } from 'react-select';

import 'react-day-picker/dist/style.css';

import styled from 'styled-components';

import { Box, Text } from 'design';
import { displayDate } from 'design/datetime';
import Dialog from 'design/DialogConfirmation';
import Select, { Option } from 'shared/components/Select';
import { useRefClickOutside } from 'shared/hooks/useRefClickOutside';

import { State } from 'teleport/Audit/useAuditEvents';

import { CustomRange } from './Custom';
import { EventRange } from './utils';

type RangeOption = Option<EventRange, string>;

export default function DataRange({ ml, range, onChangeRange, ranges }: Props) {
  const [isPickerOpen, openDayPicker] = useState(false);
  const [rangeOptions] = useState(() =>
    ranges.map(range => ({ value: range, label: range.name }))
  );

  const dayPickerRef = useRefClickOutside<HTMLDivElement>({
    open: isPickerOpen,
    setOpen: openDayPicker,
  });

  function handleOnChange(option: Option<EventRange>) {
    if (option.value.isCustom) {
      openDayPicker(true);
    } else {
      onChangeRange(option.value);
    }
  }

  function onClosePicker() {
    openDayPicker(false);
  }

  function onSetCustomRange(from: Date, to: Date) {
    onChangeRange({ isCustom: true, from, to });
    onClosePicker();
  }

  return (
    <>
      <Box ml={ml} width="210px">
        <Select
          isSearchable={false}
          components={{ ValueContainer }}
          options={rangeOptions}
          onChange={handleOnChange}
          value={{ value: range, label: range.name }}
        />
      </Box>
      <Dialog
        dialogCss={() => ({ padding: '0' })}
        disableEscapeKeyDown={false}
        onClose={onClosePicker}
        open={isPickerOpen}
      >
        <CustomRange
          initialRange={{ from: range.from, to: range.to }}
          onChange={onSetCustomRange}
          ref={dayPickerRef}
        />
      </Dialog>
    </>
  );
}

const ValueContainer = ({
  children,
  ...props
}: ValueContainerProps<RangeOption>) => {
  const { isCustom, from, to } = props.getValue()[0].value;

  if (isCustom) {
    return (
      <components.ValueContainer {...props}>
        <ValueText color="text.main">
          {`${displayDate(from)} - ${displayDate(to)}`}
        </ValueText>
        {children}
      </components.ValueContainer>
    );
  }

  return (
    <components.ValueContainer {...props}>{children}</components.ValueContainer>
  );
};

/** Positions the value text on the internal react-select grid. */
const ValueText = styled(Text)`
  grid-area: 1/1/2/3;
`;

type Props = {
  ml?: string | number;
  range: State['range'];
  onChangeRange: State['setRange'];
  ranges: State['rangeOptions'];
};
