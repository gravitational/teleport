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

import Dialog from './UserTokenLink';

export default {
  title: 'Teleport/Users/UserTokenLink',
};

export const Invite = () => {
  return <Dialog {...props} asInvite={true} />;
};

export const Reset = () => {
  return <Dialog {...props} />;
};

const props = {
  token: {
    value: '0c536179038b386728dfee6602ca297f',
    expires: new Date('2021-04-08T07:30:00Z'),
    username: 'Lester',
  },

  onClose() {},
};
