/**
 * Teleport
 * Copyright (C) 2026 Gravitational, Inc.
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

import { useHistory, useLocation } from 'react-router';

export function useNavigateBack({
  fallbackRoute,
  fallbackOptions,
}: {
  fallbackRoute: string;
  fallbackOptions?: { replace?: boolean; state?: unknown };
}) {
  const location = useLocation();
  const history = useHistory();

  return () => {
    // If location.key is unset, this is the first history entry
    // in-app in the session.
    if (!location.key) {
      if (fallbackOptions?.replace) {
        history.replace(fallbackRoute, fallbackOptions?.state);
      } else {
        history.push(fallbackRoute, fallbackOptions?.state);
      }
    } else {
      history.goBack();
    }
  };
}
