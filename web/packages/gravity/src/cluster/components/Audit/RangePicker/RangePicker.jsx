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

import React from 'react'
import moment from 'moment';
import 'react-day-picker/lib/style.css';
import { ButtonSecondary } from 'design';
import * as Icons from 'design/Icon';
import Menu, { MenuItem} from 'design/Menu';
import Dialog from 'design/DialogConfirmation';
import CustomRange from './Custom';
import { displayDate }  from 'gravity/lib/dateUtils';

export default function DataRange(props) {
  const { ml, value, onChange, disabled, options } = props;
  const { from, to, name, isCustom } = value;

  // state
  const [ isPickerOpen, openDayPicker ] = React.useState(false);
  const [ isMenuOpen, openMenu ] = React.useState(false);
  const anchorEl = React.useRef()

  function onCloseMenu(){
    openMenu(false);
  }

  function onOpenMenu(){
    openMenu(true);
  }

  function onMenuClick(option){
    openMenu(false);
    if(option.isCustom){
      openDayPicker(true);
    }else{
      onChange(option);
    }
  }

  function onClosePicker() {
    openDayPicker(false);
  }

  function onSetRange(from, to){
    onChange({ isCustom: true, from, to });
    onClosePicker();
  }

  const btnText = isCustom ?
    `${displayDate(from)} - ${displayDate(to)}` : name;

  return (
    <>
      <ButtonSecondary width="240px" disabled={disabled} ml={ml} setRef={anchorEl} onClick={onOpenMenu}>
        {btnText}
        <Icons.CarrotDown ml="3" fontSize="3" color="text.onDark"/>
      </ButtonSecondary>
      <Menu
        anchorEl={anchorEl.current}
        open={isMenuOpen}
        onClose={onCloseMenu}
        menuListCss={menuListCss}
      >
        {renderOptions(options, onMenuClick)}
      </Menu>
      <Dialog
        dialogCss={() => ({ padding: "0"}) }
        disableEscapeKeyDown={false}
        onClose={onClosePicker}
        open={isPickerOpen}
      >
        <CustomRange from={from} to={to} onChange={onSetRange}/>
      </Dialog>
    </>
  );
}

function renderOptions(options, onClick){
  return options.map(o => (
    <MenuItem key={o.name} onClick={ () => onClick(o)}>
      {o.name}
    </MenuItem>
  ))
}

const menuListCss = () => `
  width: 240px;
`

export function getRangeOptions() {
  return [
    {
      name: 'Today',
      from: moment(new Date()).startOf('day').toDate(),
      to: moment(new Date()).endOf('day').toDate(),
    },
    {
      name: '7 days',
      from: moment().subtract(6, 'day').startOf('day').toDate(),
      to: moment(new Date()).endOf('day').toDate(),
    },
    {
      name: 'Custom Range...',
      isCustom: true,
      from: new Date(),
      to: new Date(),
    }
  ]
}