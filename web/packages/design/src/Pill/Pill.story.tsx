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

import { Pill } from './Pill';

export default {
  title: 'Design/Pill',
};

export const PillOptions = () => {
  return (
    <>
      <Pill label="arch: x86_64" />
      <br />
      <br />
      <Pill label="hostname: ip-172-31-9-155.us-west-2.compute.internal" />
      <br />
      <br />
      <Pill label="arch: x86_64" onDismiss={() => {}} />
      <br />
      <br />
      <Pill
        label="hostname: ip-172-31-9-155.us-west-2.compute.internal"
        onDismiss={() => {}}
      />
    </>
  );
};
