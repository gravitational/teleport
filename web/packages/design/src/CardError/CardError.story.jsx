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

import * as CardError from './CardError';

const message = 'some error message';

export default {
  title: 'Design/Card/CardError',
};

export const Cards = () => (
  <>
    <CardError.NotFound message={message} />
    <CardError.AccessDenied message={message} />
    <CardError.Failed message={message} />
    <CardError.LoginFailed message={message} loginUrl="https://localhost" />
    <CardError.Offline
      title={'This cluster is not available from Teleport.'}
      message={'To access this cluster, please use its local endpoint'}
    />
  </>
);
