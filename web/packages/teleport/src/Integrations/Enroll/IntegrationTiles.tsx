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
import { Text, Box } from 'design';
import { AWSIcon } from 'design/SVGIcon';

import cfg from 'teleport/config';
import {
  BadgeTitle,
  ToolTipNoPermBadge,
} from 'teleport/components/ToolTipNoPermBadge';
import { IntegrationKind } from 'teleport/services/integrations';

import { ToolTipBadge } from 'teleport/components/ToolTipBadge';

import { IntegrationTile } from './common';

export function IntegrationTiles({
  hasIntegrationAccess = true,
  hasExternalAuditStorage = true,
}: {
  hasIntegrationAccess?: boolean;
  hasExternalAuditStorage?: boolean;
}) {
  const externalAuditStorageEnabled = cfg.externalAuditStorage;
  const isOnpremEnterprise = cfg.isEnterprise && !cfg.isCloud;

  return (
    <>
      <IntegrationTile
        disabled={!hasIntegrationAccess}
        as={hasIntegrationAccess ? Link : null}
        to={
          hasIntegrationAccess
            ? cfg.getIntegrationEnrollRoute(IntegrationKind.AwsOidc)
            : null
        }
        data-testid="tile-aws-oidc"
      >
        <Box mt={3} mb={2}>
          <AWSIcon size={80} />
        </Box>
        <Text>
          Amazon Web Services
          <br />
          OIDC
        </Text>
        {!hasIntegrationAccess && (
          <ToolTipNoPermBadge
            children={
              <div>
                You don’t have sufficient permissions to create an integration.
                Reach out to your Teleport administrator to request additional
                permissions.
              </div>
            }
          />
        )}
      </IntegrationTile>
      {!isOnpremEnterprise && (
        <IntegrationTile
          disabled={!hasExternalAuditStorage || !externalAuditStorageEnabled}
          as={hasExternalAuditStorage ? Link : null}
          to={
            hasExternalAuditStorage
              ? cfg.getIntegrationEnrollRoute(
                  IntegrationKind.ExternalAuditStorage
                )
              : null
          }
          data-testid="tile-external-audit-storage"
        >
          <Box mt={3} mb={2}>
            <AWSIcon size={80} />
          </Box>
          <Text>AWS External Audit Storage</Text>
          {renderExternalAuditStorageBadge(
            hasExternalAuditStorage,
            externalAuditStorageEnabled
          )}
        </IntegrationTile>
      )}
    </>
  );
}

function renderExternalAuditStorageBadge(
  hasExternalAuditStorageAccess: boolean,
  isEnterprise: boolean
) {
  if (!isEnterprise)
    return (
      <ToolTipNoPermBadge
        badgeTitle={BadgeTitle.LackingEnterpriseLicense}
        children={
          <div>Unlock External Audit Storage with Teleport Enterprise</div>
        }
      />
    );
  if (!hasExternalAuditStorageAccess) {
    return (
      <ToolTipNoPermBadge
        children={
          <div>
            You don’t have sufficient permissions to create an External Audit
            Storage. Reach out to your Teleport administrator to request
            additional permissions.
          </div>
        }
      />
    );
  }

  return (
    <ToolTipBadge
      badgeTitle="New"
      children={
        <div>
          Connect your own AWS account to store Audit logs and Session
          recordings using Athena and S3.
        </div>
      }
      color="success.main"
    />
  );
}
