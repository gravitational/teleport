/**
 * Teleport
 * Copyright (C) 2024 Gravitational, Inc.
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

import { BannerList } from './BannerList';

export default {
  title: 'Teleport/BannerList',
};

export function List() {
  return (
    <BannerList
      banners={[
        { id: 'ban1', severity: 'info', message: 'This is fine.' },
        {
          id: 'ban2',
          severity: 'warning',
          message: 'Click this, or else',
          linkText: 'Click Me',
          linkDestination: 'https://goteleport.com/',
        },
        {
          id: 'ban3',
          severity: 'danger',
          message: 'External link',
          linkText: 'Click Me',
          linkDestination: 'https://example.com/',
        },
        {
          id: 'ban4',
          severity: 'danger',
          message: 'Default link text',
          linkDestination: 'https://goteleport.com/',
        },
        {
          id: 'ban5',
          severity: 'danger',
          message: 'External link, default link text',
          linkDestination: 'https://google.com/',
        },
      ]}
    />
  );
}
