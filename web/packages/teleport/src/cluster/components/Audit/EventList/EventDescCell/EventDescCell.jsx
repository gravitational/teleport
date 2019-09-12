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
import { ButtonPrimary } from 'design/Button';
import { CirclePlay } from 'design/Icon';
import { CodeEnum } from 'teleport/services/events/event';
import cfg from 'teleport/config';

function getDescription({ code, message, details }) {
  switch (code) {
    case CodeEnum.SESSION_END:
      const { sid } = details;
      const url = cfg.getConsolePlayerRoute({ sid });
      return (
        <>
          <ButtonPrimary mr="2" size="small" as="a" href={url} target="_blank">
            <CirclePlay mr="1" fontSize={1} />
            Play
          </ButtonPrimary>
          {message}
        </>
      );
  }

  return message;
}

export default function TypeCell({ rowIndex, data }) {
  const desc = getDescription(data[rowIndex]);
  return <Cell>{desc}</Cell>;
}
