/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import { IconTooltip } from '../Tooltip';
import { SyncStamp } from './SyncStamp';

export default {
  title: 'Design/SyncStamp',
};

export const Story = () => (
  <>
    <SyncStamp date={new Date()} />
    <SyncStamp date={new Date('2019-08-30T00:00:00.00Z')} />
    <SyncStamp date={new Date()}>
      <IconTooltip kind="error">Failed</IconTooltip>
    </SyncStamp>
  </>
);
