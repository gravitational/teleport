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
import { Cell } from 'design/DataTable';
import { ButtonBorder } from 'design';
import { displayDateTime } from 'shared/services/loc';

import { Event } from 'teleport/services/audit';

export const ActionCell = props => {
  const { rowIndex, onViewDetails, data } = props;
  const event: Event = data[rowIndex];

  function onClick() {
    onViewDetails(event);
  }
  return (
    <Cell align="right">
      <ButtonBorder size="small" onClick={onClick} width="87px">
        Details
      </ButtonBorder>
    </Cell>
  );
};

export const TimeCell = props => {
  const { rowIndex, data } = props;
  const { time } = data[rowIndex];
  return <Cell style={{ minWidth: '120px' }}>{displayDateTime(time)}</Cell>;
};

export function DescCell(props) {
  const { rowIndex, data } = props;
  const { message } = data[rowIndex];
  return <Cell style={{ wordBreak: 'break-word' }}>{message}</Cell>;
}
