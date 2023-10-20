/**
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react';
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
