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

export default function useFavicon(path: string) {
  React.useEffect(() => {
    const link: HTMLLinkElement =
      document.querySelector("link[rel*='icon']") ||
      document.createElement('link');

    link.href = path;
    link.rel = 'shortcut icon';
    link.type = 'image/x-icon';

    document.getElementsByTagName('head')[0].appendChild(link);
  }, [path]);

  return null;
}
