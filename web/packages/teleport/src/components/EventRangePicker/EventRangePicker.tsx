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

import React from 'react';
import moment from 'moment';
import 'react-day-picker/lib/style.css';
import { ButtonOutlined } from 'design';
import { CarrotDown } from 'design/Icon';
import Menu, { MenuItem } from 'design/Menu';
import Dialog from 'design/DialogConfirmation';
import { displayDate } from 'shared/services/loc';
import CustomRange from './Custom';

export default function DataRange(props) {
  const { ml, value, onChange, disabled, options } = props;
  const { from, to, name, isCustom } = value;

  // state
  const [isPickerOpen, openDayPicker] = React.useState(false);
  const [isMenuOpen, openMenu] = React.useState(false);
  const anchorEl = React.useRef();

  function onCloseMenu() {
    openMenu(false);
  }

  function onOpenMenu() {
    openMenu(true);
  }

  function onMenuClick(option) {
    openMenu(false);
    if (option.isCustom) {
      openDayPicker(true);
    } else {
      onChange(option);
    }
  }

  function onClosePicker() {
    openDayPicker(false);
  }

  function onSetRange(from, to) {
    onChange({ isCustom: true, from, to });
    onClosePicker();
  }

  const btnText = isCustom ? `${displayDate(from)} - ${displayDate(to)}` : name;

  return (
    <>
      <ButtonOutlined
        size="small"
        width="auto"
        disabled={disabled}
        ml={ml}
        setRef={anchorEl}
        onClick={onOpenMenu}
        style={{ minWidth: '100px' }}
      >
        {btnText}
        <CarrotDown ml={2} mr={-2} fontSize="2" color="text.primary" />
      </ButtonOutlined>
      <Menu
        anchorEl={anchorEl.current}
        open={isMenuOpen}
        onClose={onCloseMenu}
        menuListCss={() => ({ width: '200px' })}
      >
        {renderOptions(options, onMenuClick)}
      </Menu>
      <Dialog
        dialogCss={() => ({ padding: '0' })}
        disableEscapeKeyDown={false}
        onClose={onClosePicker}
        open={isPickerOpen}
      >
        <CustomRange
          from={from}
          to={to}
          onChange={onSetRange}
          onClosePicker={onClosePicker}
        />
      </Dialog>
    </>
  );
}

function renderOptions(options, onClick) {
  return options.map(o => (
    <MenuItem key={o.name} onClick={() => onClick(o)}>
      {o.name}
    </MenuItem>
  ));
}

export function getRangeOptions() {
  return [
    {
      name: 'Today',
      from: moment(new Date())
        .startOf('day')
        .toDate(),
      to: moment(new Date())
        .endOf('day')
        .toDate(),
    },
    {
      name: '7 days',
      from: moment()
        .subtract(6, 'day')
        .startOf('day')
        .toDate(),
      to: moment(new Date())
        .endOf('day')
        .toDate(),
    },
    {
      name: 'Custom Range...',
      isCustom: true,
      from: new Date(),
      to: new Date(),
    },
  ];
}
