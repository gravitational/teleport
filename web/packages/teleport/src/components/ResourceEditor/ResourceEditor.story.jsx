/*
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

import ResourceEditor from './ResourceEditor';

export default {
  title: 'Teleport/ResourceEditor',
};

export const New = () => (
  <ResourceEditor
    text="sample text"
    directions=""
    docsURL=""
    onClose={() => null}
    isNew={true}
    title="Creating a new resource"
    name="sample"
    onSave={() => Promise.reject(new Error('server error'))}
  />
);

export const Edit = () => (
  <ResourceEditor
    text="sample text"
    directions=""
    docsURL=""
    onClose={() => null}
    isNew={false}
    title="Creating a new resource"
    name="sample"
    onSave={() => Promise.reject(new Error('server error'))}
  />
);

export const Directions = () => (
  <ResourceEditor
    docsURL="localhost"
    directions={<div>Custom text here</div>}
    title="Modifying the resource"
    name="sample"
    text="some text"
    onClose={() => null}
    isNew={false}
    onSave={() => Promise.reject(new Error('server error'))}
  />
);
