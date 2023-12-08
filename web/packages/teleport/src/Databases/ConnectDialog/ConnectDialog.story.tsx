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

import React from 'react';

import ConnectDialog from './ConnectDialog';

export default {
  title: 'Teleport/Databases/Connect',
};

export const Connect = () => (
  <ConnectDialog
    username="yassine"
    dbName="aurora"
    dbProtocol="postgres"
    clusterId="im-a-cluster"
    onClose={() => null}
    authType="local"
  />
);

export const ConnectWithRequestId = () => {
  return (
    <ConnectDialog
      username="yassine"
      dbName="aurora"
      dbProtocol="postgres"
      clusterId="im-a-cluster"
      onClose={() => null}
      authType="local"
      accessRequestId="e1e8072c-1eb8-5df4-a7bd-b6863b19271c"
    />
  );
};
