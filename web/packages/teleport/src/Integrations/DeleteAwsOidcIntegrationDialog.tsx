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

import { Alert, ButtonSecondary, ButtonWarning, P1, Text } from 'design';
import Dialog, {
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from 'design/DialogConfirmation';
import useAttempt from 'shared/hooks/useAttemptNext';

import { IntegrationAwsOidc } from 'teleport/services/integrations';
import useStickyClusterId from 'teleport/useStickyClusterId';

import { type DeleteRequestOptions } from './Operations/IntegrationOperations';

type Props = {
  close(): void;
  remove(opt?: DeleteRequestOptions): Promise<void>;
  integration: IntegrationAwsOidc;
};

export function DeleteAwsOidcIntegrationDialog(props: Props) {
  const { close, remove, integration } = props;
  const { attempt, run } = useAttempt();
  const isDisabled = attempt.status === 'processing';
  const { clusterId } = useStickyClusterId();

  // https://resource-explorer.console.aws.amazon.com
  // /resource-explorer/home#/search?query=
  // tag:teleport.dev/origin=integration_awsoidc+
  // tag:teleport.dev/cluster={cluster_name}+
  // tag:teleport.dev/integration={integration_name}
  const awsResourceExplorerUrl =
    `https://resource-explorer.console.aws.amazon.com` +
    `/resource-explorer/home#/search?query=` +
    `tag%3Ateleport.dev%2Forigin%3Dintegration_awsoidc+` +
    `tag%3Ateleport.dev%2Fcluster%3D${clusterId}+` +
    `tag%3Ateleport.dev%2Fintegration%3D${integration.name}`;
  function onOk() {
    run(() => remove({ deleteAssociatedResources: true }));
  }

  return (
    <Dialog onClose={close} open={true}>
      <DialogHeader>
        <DialogTitle>Delete AWS OIDC Integration?</DialogTitle>
      </DialogHeader>
      <DialogContent width="600px">
        {attempt.status === 'failed' && <Alert children={attempt.statusText} />}
        <P1 mb={3}>
          Are you sure you want to delete integration{' '}
          <Text as="span" bold color="text.main">
            {integration.name}
          </Text>
          ?
        </P1>
        <Alert
          kind="warning"
          primaryAction={{
            content: 'AWS Resource Explorer',
            href: awsResourceExplorerUrl,
          }}
        >
          AWS resources created by this integration may require manual clean up.
          Visit AWS Resource Explorer to see AWS resources tagged by this
          integration.
        </Alert>
        <Alert kind="warning">
          Teleport will also remove App servers and resources used for
          auto-discovery that reference this integration.
        </Alert>
      </DialogContent>
      <DialogFooter>
        <ButtonWarning mr="3" disabled={isDisabled} onClick={onOk}>
          Yes, Delete Integration
        </ButtonWarning>
        <ButtonSecondary disabled={isDisabled} onClick={close}>
          Cancel
        </ButtonSecondary>
      </DialogFooter>
    </Dialog>
  );
}
