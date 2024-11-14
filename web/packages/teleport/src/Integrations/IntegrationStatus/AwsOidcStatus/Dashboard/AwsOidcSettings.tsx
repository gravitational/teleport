/**
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

import React, { useState } from 'react';
import { useHistory } from 'react-router-dom';
import { Flex, MenuItem } from 'design';
import { Cog } from 'design/Icon';
import { MenuIcon } from 'shared/components/MenuAction';

import {
  Integration,
  integrationService,
} from 'teleport/services/integrations';
import cfg from 'teleport/config';
import { DeleteIntegrationDialog } from 'teleport/Integrations/RemoveIntegrationDialog';
import { EditAwsOidcIntegrationDialog } from 'teleport/Integrations/EditAwsOidcIntegrationDialog';
import { EditableIntegrationFields } from 'teleport/Integrations/Operations/useIntegrationOperation';

export function AwsOidcSettings({ integration }: { integration: Integration }) {
  const history = useHistory();

  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [showEditDialog, setShowEditDialog] = useState(false);

  function deleteIntegration() {
    return integrationService.deleteIntegration(integration.name).then(() => {
      // redirect to integration listing page after deletion
      history.push(cfg.routes.integrations);
    });
  }

  function editIntegration(req: EditableIntegrationFields) {
    return integrationService
      .updateIntegration(integration.name, {
        awsoidc: { roleArn: req.roleArn },
      })
      .then(() => {
        setShowEditDialog(false);
      });
  }

  return (
    <>
      <Flex alignItems="center">
        <MenuIcon
          Icon={Cog}
          // menuProps={{
          //   anchorOrigin: { vertical: 'top', horizontal: 'left' },
          //   transformOrigin: { vertical: 'top', horizontal: 'left' },
          // }}
        >
          <MenuItem onClick={() => setShowEditDialog(true)}>Edit…</MenuItem>
          <MenuItem onClick={() => setShowDeleteDialog(true)}>Delete…</MenuItem>
        </MenuIcon>
      </Flex>
      {showDeleteDialog && (
        <DeleteIntegrationDialog
          close={() => setShowDeleteDialog(false)}
          remove={deleteIntegration}
          name={integration.name}
        />
      )}
      {showEditDialog && (
        <EditAwsOidcIntegrationDialog
          close={() => setShowEditDialog(false)}
          edit={editIntegration}
          integration={integration}
        />
      )}
    </>
  );
}
