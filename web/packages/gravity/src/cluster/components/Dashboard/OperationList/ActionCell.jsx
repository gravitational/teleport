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
import PropTypes from 'prop-types';
import { NavLink } from 'gravity/components/Router';
import cfg from 'gravity/config';
import { Cell } from 'design/DataTable';
import { ButtonPrimary, ButtonSecondary } from 'design';

export default function ActionCell({ logsEnabled, rowIndex, data }) {
  const { isSession, session, operation } = data[rowIndex];
  if (isSession) {
    return renderSessionCell(session);
  }

  if (logsEnabled) {
    return renderOperationCell(operation);
  }

  return null;
}

ActionCell.propTypes = {
  logsEnabled: PropTypes.bool.isRequired,
};

function renderSessionCell(session) {
  const { sid } = session;
  const url = cfg.getConsoleSessionRoute({ sid });
  return (
    <Cell align="right">
      <ButtonPrimary as="a" target="_blank" href={url} size="small" width="90px" children="join" />
    </Cell>
  );
}

function renderOperationCell(operation) {
  const { id } = operation;
  const url = cfg.getSiteLogQueryRoute({ query: `file:${id}` });
  return (
    <Cell align="right">
      <ButtonSecondary as={NavLink} to={url} size="small" width="90px">
        VIEW LOGS
      </ButtonSecondary>
    </Cell>
  );
}
