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
import Label from 'design/Label';
import { Cell } from 'design/DataTable';

export function NameCell({ rowIndex, data }){
  const { name } = data[rowIndex];
  return (
    <Cell style={{fontSize: "14px"}}>
      {name}
    </Cell>
  )
}

export function PortCell({ rowIndex, data }){
  const service = data[rowIndex];
  const $ports = service.ports.map(text => {
    const [port1, port2] = text.split('/');
    return (
      <div key={text}>
        {port1}
        {port2}
      </div>
    );
  });

  return (<Cell>{$ports}</Cell>);
}

export function LabelCell({ rowIndex, data }){
  const service = data[rowIndex];
  const $labelItems = service.labels.map( (text, key) => (
    <Label kind="secondary" mb="1" mr="1" key={key} children={text} />
    )
 );

  return (<Cell> {$labelItems} </Cell>);
}