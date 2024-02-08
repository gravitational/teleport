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

import { useCallback } from 'react';
import { ButtonPrimary, ButtonSecondary } from 'design/Button';
import { useAsync } from 'shared/hooks/useAsync';
import { Flex, Box } from 'design';

import Document from 'teleterm/ui/Document';

import { useAppContext } from 'teleterm/ui/appContextProvider';

import type * as docTypes from 'teleterm/ui/services/workspacesService';

export function DocumentVnet(props: {
  visible: boolean;
  doc: docTypes.DocumentVnet;
}) {
  const { doc } = props;
  const { tshd } = useAppContext();

  const [startAttempt, startVnet] = useAsync(
    useCallback(
      () => tshd.startVnet(doc.rootClusterUri),
      [tshd, doc.rootClusterUri]
    )
  );

  const [stopAttempt, stopVnet] = useAsync(
    useCallback(
      () => tshd.stopVnet(doc.rootClusterUri),
      [tshd, doc.rootClusterUri]
    )
  );

  return (
    <Document visible={props.visible}>
      <Box p={3}>
        <Flex gap={2}>
          <ButtonPrimary
            onClick={startVnet}
            disabled={
              startAttempt.status === 'processing' ||
              startAttempt.status === 'success'
            }
          >
            Start
          </ButtonPrimary>
          <ButtonSecondary
            onClick={stopVnet}
            disabled={
              startAttempt.status !== 'success' ||
              stopAttempt.status === 'processing' ||
              stopAttempt.status === 'success'
            }
          >
            Stop
          </ButtonSecondary>
        </Flex>
      </Box>
    </Document>
  );
}
