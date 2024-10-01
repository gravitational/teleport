/**
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
import { useEffect, useState } from 'react';

import useAttempt from 'shared/hooks/useAttemptNext';

import { Resource } from 'teleport/services/resources';
import useTeleport from 'teleport/useTeleport';

export default function useGitHub() {
  const ctx = useTeleport();
  const [connectors, setConnectors] = useState<Resource<'github'>[]>([]);
  const { attempt, run } = useAttempt('processing');

  function fetchGithubConnectors(){
    return ctx.resourceService.fetchGithubConnectors().then(response => {
      setConnectors(response);
    });
  }

  useEffect(() => {
    run(() => fetchGithubConnectors());
  }, []);

  return {
    connectors,
    attempt,
  };
}

export type State = ReturnType<typeof useGitHub>;
