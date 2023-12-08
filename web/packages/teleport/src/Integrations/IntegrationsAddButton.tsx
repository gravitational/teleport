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

import React from 'react';
import { Link } from 'react-router-dom';
import { ButtonPrimary } from 'design';

import cfg from 'teleport/config';

export function IntegrationsAddButton({
  canCreate = false,
}: {
  canCreate: boolean;
}) {
  return (
    <ButtonPrimary
      as={Link}
      ml="auto"
      width="240px"
      disabled={!canCreate}
      to={cfg.getIntegrationEnrollRoute()}
      title={canCreate ? '' : 'You do not have access to add new integrations'}
    >
      Enroll new integration
    </ButtonPrimary>
  );
}
