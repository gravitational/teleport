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

import Component from './RecoveryCodes';

export default {
  title: 'Teleport/RecoveryCodes',
};

export const FromInvite = () => <Component {...props} />;

export const FromReset = () => (
  <Component {...props} isNewCodes={true} continueText="Return to login" />
);

const props = {
  recoveryCodes: {
    codes: [
      'tele-testword-testword-testword-testword-testword-testword-testword',
      'tele-testword-testword-testword-testword-testword-testword-testword-testword',
      'tele-testword-testword-testword-testword-testword-testword-testword',
    ],
    createdDate: new Date('2019-08-30T11:00:00.00Z'),
  },
  onContinue: () => null,
  isNewCodes: false,
};
