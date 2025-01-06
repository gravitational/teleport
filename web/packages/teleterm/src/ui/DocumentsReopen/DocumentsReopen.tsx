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

import { ButtonIcon, ButtonPrimary, ButtonSecondary, H2 } from 'design';
import DialogConfirmation, {
  DialogContent,
  DialogFooter,
  DialogHeader,
} from 'design/DialogConfirmation';
import { Cross } from 'design/Icon';
import { P } from 'design/Text/Text';
import { pluralize } from 'shared/utils/text';

import { useAppContext } from 'teleterm/ui/appContextProvider';
import { RootClusterUri, routing } from 'teleterm/ui/uri';

export function DocumentsReopen(props: {
  rootClusterUri: RootClusterUri;
  numberOfDocuments: number;
  onDiscard(): void;
  onConfirm(): void;
  hidden?: boolean;
}) {
  const { rootClusterUri } = props;
  const { clustersService } = useAppContext();
  // TODO(ravicious): Use a profile name here from the URI and remove the dependency on
  // clustersService. https://github.com/gravitational/teleport/issues/33733
  const clusterName =
    clustersService.findCluster(rootClusterUri)?.name ||
    routing.parseClusterName(rootClusterUri);

  return (
    <DialogConfirmation
      open={!props.hidden}
      keepInDOMAfterClose
      onClose={props.onDiscard}
      dialogCss={() => ({
        maxWidth: '400px',
        width: '100%',
      })}
    >
      <form
        onSubmit={e => {
          e.preventDefault();
          props.onConfirm();
        }}
      >
        <DialogHeader
          justifyContent="space-between"
          mb={0}
          alignItems="baseline"
        >
          <H2 mb={4}>Reopen previous session</H2>
          <ButtonIcon
            type="button"
            onClick={props.onDiscard}
            title="Close"
            color="text.slightlyMuted"
          >
            <Cross size="medium" />
          </ButtonIcon>
        </DialogHeader>
        <DialogContent mb={4}>
          <P color="text.slightlyMuted">
            Do you want to reopen tabs from the previous session?
          </P>
          <P
            color="text.slightlyMuted"
            // Split long continuous cluster names into separate lines.
            css={`
              word-wrap: break-word;
            `}
          >
            {/*
              We show this mostly because we needed to show the cluster name somewhere during UI
              initialization. When you open the app and have some tabs to restore, the UI will show
              nothing else but this modal. Showing the cluster name provides some information to the
              user about which workspace they're in.
            */}
            You had{' '}
            <strong>
              {props.numberOfDocuments}{' '}
              {pluralize(props.numberOfDocuments, 'tab')}
            </strong>{' '}
            open in <strong>{clusterName}</strong>.
          </P>
        </DialogContent>
        <DialogFooter>
          <ButtonPrimary autoFocus mr={3} type="submit">
            Reopen
          </ButtonPrimary>
          <ButtonSecondary type="button" onClick={props.onDiscard}>
            Start New Session
          </ButtonSecondary>
        </DialogFooter>
      </form>
    </DialogConfirmation>
  );
}
