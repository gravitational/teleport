/*
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

import React from 'react';

import { Restore } from 'design/Icon';

import { Box } from '..';

import { Alert, AlertProps } from './Alert';

export default {
  title: 'Design/Alerts',
};

export const Simple = () => (
  <Box maxWidth="600px">
    <Alert kind="neutral">Some neutral message</Alert>
    <Alert kind="danger">Some error message</Alert>
    <Alert kind="warning">Some warning message</Alert>
    <Alert kind="info">Some informational message</Alert>
    <Alert kind="success">This is success</Alert>
    <Alert kind="neutral" icon={Restore}>
      Alert with a custom icon
    </Alert>
  </Box>
);

export const Actionable = () => (
  <Box maxWidth="700px">
    <Alert kind="neutral" {...commonProps}>
      Some neutral message
    </Alert>
    <Alert kind="danger" {...commonProps}>
      Some error message
    </Alert>
    <Alert kind="warning" {...commonProps}>
      Some warning message
    </Alert>
    <Alert kind="info" {...commonProps}>
      Some informational message
    </Alert>
    <Alert kind="success" {...commonProps}>
      This is success
    </Alert>
    <Alert
      kind="info"
      {...commonProps}
      details="AllworkandnoplaymakesJackadullboy.AllworkandnoplaymakesJackadullboy."
    >
      All work and no play makes Jack a dull boy. All work and no play makes
      Jack a dull boy.
      AllworkandnoplaymakesJackadullboy.AllworkandnoplaymakesJackadullboy.AllworkandnoplaymakesJackadullboy.AllworkandnoplaymakesJackadullboy.AllworkandnoplaymakesJackadullboy.
    </Alert>
  </Box>
);

const commonProps: AlertProps = {
  details: 'Message subtitle',
  dismissible: true,
  primaryAction: {
    content: 'Primary Action',
    onClick: () => {
      alert('Primary button clicked');
    },
  },
  secondaryAction: {
    content: 'Secondary Action',
    onClick: () => {
      alert('Secondary button clicked');
    },
  },
};
