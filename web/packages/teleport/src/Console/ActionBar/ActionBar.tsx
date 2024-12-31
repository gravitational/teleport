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

import { Flex } from 'design';
import { LatencyDiagnostic } from 'shared/components/LatencyDiagnostic';

import { DocumentSsh } from 'teleport/Console/stores';

export default function ActionBar(props: Props) {
  return (
    <Flex alignItems="center">
      {props.latencyIndicator.isVisible && (
        <LatencyDiagnostic latency={props.latencyIndicator.latency} />
      )}
    </Flex>
  );
}

type Props = {
  latencyIndicator:
    | { isVisible: true; latency: DocumentSsh['latency'] }
    | {
        isVisible: false;
      };
};
