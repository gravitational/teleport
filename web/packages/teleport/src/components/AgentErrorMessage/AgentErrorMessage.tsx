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

import { Link } from 'design';
import { Danger } from 'design/Alert';

const PREDICATE_DOC =
  'https://goteleport.com/docs/setup/reference/predicate-language/#resource-filtering';

export default function AgentErrorMessage({ message = '' }) {
  const showDocLink = message.includes('predicate expression');

  return (
    <Danger>
      <div>
        {message}
        {showDocLink && (
          <>
            , click{' '}
            <Link target="_blank" href={PREDICATE_DOC}>
              here
            </Link>{' '}
            for syntax examples
          </>
        )}
      </div>
    </Danger>
  );
}
