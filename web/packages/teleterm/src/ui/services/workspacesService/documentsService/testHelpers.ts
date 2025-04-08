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

import { makeRootCluster } from 'teleterm/services/tshd/testHelpers';

import { DocumentCluster } from './types';

export function makeDocumentCluster(
  props?: Partial<DocumentCluster>
): DocumentCluster {
  return {
    kind: 'doc.cluster',
    uri: '/docs/unique-uri',
    title: 'teleport-ent.asteroid.earth',
    clusterUri: makeRootCluster().uri,
    queryParams: {
      sort: {
        fieldName: 'name',
        dir: 'ASC',
      },
      resourceKinds: [],
      search: '',
      advancedSearchEnabled: false,
    },
    ...props,
  };
}
