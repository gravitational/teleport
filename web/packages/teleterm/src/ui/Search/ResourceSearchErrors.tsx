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

import { ButtonIcon, ButtonSecondary, H2, Text } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';

import { ResourceSearchError } from 'teleterm/ui/services/resources';
import type * as uri from 'teleterm/ui/uri';

export function ResourceSearchErrors(props: {
  errors: ResourceSearchError[];
  getClusterName: (resourceUri: uri.ClusterOrResourceUri) => string;
  onCancel: () => void;
  hidden?: boolean;
}) {
  const formattedErrorText = props.errors
    .map(error => error.messageAndCauseWithClusterName(props.getClusterName))
    .join('\n\n');

  return (
    <DialogConfirmation
      open={!props.hidden}
      keepInDOMAfterClose
      onClose={props.onCancel}
      dialogCss={() => ({
        maxWidth: '800px',
        width: '100%',
      })}
    >
      <DialogHeader justifyContent="space-between" alignItems="baseline">
        <H2>Resource search errors</H2>
        <ButtonIcon
          type="button"
          onClick={props.onCancel}
          color="text.slightlyMuted"
        >
          <Cross size="medium" />
        </ButtonIcon>
      </DialogHeader>
      <DialogContent mb={4}>
        <Text typography="body2" color="text.slightlyMuted">
          <pre
            css={`
              padding: ${props => props.theme.space[2]}px;
              background-color: ${props => props.theme.colors.levels.sunken};
              color: ${props => props.theme.colors.text.main};

              white-space: pre-wrap;
              max-height: calc(${props => props.theme.space[6]}px * 10);
              overflow-y: auto;
            `}
          >
            {formattedErrorText}
          </pre>
        </Text>
      </DialogContent>
      <DialogFooter>
        <ButtonSecondary type="button" onClick={props.onCancel}>
          Close
        </ButtonSecondary>
      </DialogFooter>
    </DialogConfirmation>
  );
}
