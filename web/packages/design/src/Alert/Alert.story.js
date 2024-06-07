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

import { Box } from './../';

import Alert from './index';

export default {
  title: 'Design/Alerts',
};

export const Alerts = () => (
  <Box maxWidth="600px">
    <Alert kind="danger">Some error message</Alert>
    <Alert kind="warning">Some warning message</Alert>
    <Alert kind="info">Some informational message</Alert>
    <Alert kind="success">This is success</Alert>
    <Alert kind="outline-info">Text align it yourself</Alert>
    <Alert kind="outline-warn">Text align it yourself</Alert>
    <Alert kind="outline-danger">Text align it yourself</Alert>
  </Box>
);
