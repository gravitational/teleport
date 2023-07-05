/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React, { useState } from 'react';
import { components } from 'react-select';
import 'react-day-picker/lib/style.css';
import { Text } from 'design';
import Dialog from 'design/DialogConfirmation';
import { displayDate } from 'shared/services/loc';

import Select, { Option, DarkStyledSelect } from 'shared/components/Select';

import { State } from 'teleport/Audit/useAuditEvents';

import CustomRange from './Custom';
import { EventRange } from './utils';

export default function DataRange({ ml, range, onChangeRange, ranges }: Props) {
  const [isPickerOpen, openDayPicker] = useState(false);
  const [rangeOptions] = useState(() =>
    ranges.map(range => ({ value: range, label: range.name }))
  );

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
      <DarkStyledSelect ml={ml} width="210px">
        <Select
          isSearchable={false}
          components={{ ValueContainer }}
          options={rangeOptions}
          onChange={handleOnChange}
          value={{ value: range, label: range.name }}
        />
      </DarkStyledSelect>
      <Dialog
        dialogCss={() => ({ padding: '0' })}
        disableEscapeKeyDown={false}
        onClose={onClosePicker}
        open={isPickerOpen}
      >
        <CustomRange
          from={range.from}
          to={range.to}
          onChange={onSetCustomRange}
          onClosePicker={onClosePicker}
        />
      </Dialog>
    </>
  );
}

const ValueContainer = ({ children, ...props }) => {
  const { isCustom, from, to } = props.getValue()[0].value;

  if (isCustom) {
    return (
      <components.ValueContainer {...props}>
        <Text color="text.main">
          {`${displayDate(from)} - ${displayDate(to)}`}
        </Text>
        {children}
      </components.ValueContainer>
    );
  }

  return (
    <components.ValueContainer {...props}>{children}</components.ValueContainer>
  );
};

type Props = {
  ml?: string | number;
  range: State['range'];
  onChangeRange: State['setRange'];
  ranges: State['rangeOptions'];
};
