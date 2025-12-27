/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

import {
  ExternalAuditStorageIntegration,
  Integration,
  Plugin,
} from 'teleport/services/integrations';

export type BaseIntegration = (
  | { name: string; title?: never } // Plugin, IntegrationTileSpec
  | { title: string; name?: never } // BotIntegration uses title
) & {
  tags: IntegrationTag[];
  description?: string;
};

export type IntegrationLike = (
  | Integration
  | Plugin
  | ExternalAuditStorageIntegration
) & { tags?: string[] };

export const integrationTags = {
  bot: 'Bot',
  cicd: 'CI/CD',
  devicetrust: 'Device Trust',
  idp: 'IdP',
  notifications: 'Notifications',
  resourceaccess: 'Resource Access',
  scim: 'SCIM',
} as const;

export enum Status {
  Success,
  Warning,
  Error,
  OktaConfigError = 20,
}

/**
 * Type representing tags used for categorizing and filtering integrations
 */
export type IntegrationTag = keyof typeof integrationTags;

export const integrationTagOptions = (
  Object.entries(integrationTags) as [IntegrationTag, string][]
).map(([value, label]) => ({ value, label }));

export function isIntegrationTag(tag: unknown): tag is IntegrationTag {
  return typeof tag === 'string' && tag in integrationTags;
}

export function getIntegrationTagLabel(t: IntegrationTag): string {
  return integrationTags[t];
}
