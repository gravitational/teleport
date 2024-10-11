/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import { useLocation } from 'react-router';

import CardIcon from 'design/CardIcon';
import { Terminal } from 'design/Icon';

export function CardTerminal() {
  const { search } = useLocation();
  const queryParams = new URLSearchParams(search);
  let provider = '';
  if (queryParams.has('auth')) {
    provider = ` by ${queryParams.get('auth')}`;
  }
  return (
    <CardIcon
      title="Continue in Terminal"
      icon={<Terminal mb={3} size={64} color="accent.main" />}
    >
      You have been authenticated{provider}.<br />
      You can close this window and finish logging in at your terminal.
    </CardIcon>
  );
}
