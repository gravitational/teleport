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

export type BaseIntegration = (
  | { name: string; title?: never }
  | { title: string; name?: never }
) & {
  tags: IntegrationTag[];
};

export const integrationTagOptions = [
  { value: 'ai', label: 'AI / MCP' },
  { value: 'bot', label: 'Bot' },
  { value: 'cicd', label: 'CI/CD' },
  { value: 'devicetrust', label: 'Device Trust' },
  { value: 'idp', label: 'IdP' },
  { value: 'jit', label: 'JIT Review' },
  { value: 'multicloud', label: 'Multi-Cloud' },
  { value: 'notifications', label: 'Notifications' },
  { value: 'resourceaccess', label: 'Resource Access' },
] as const satisfies { value: string; label: string }[];

export type IntegrationTag = Extract<
  (typeof integrationTagOptions)[number],
  { value: string }
>['value'];

export function isIntegrationTag(tag: unknown): tag is IntegrationTag {
  return (
    typeof tag === 'string' &&
    integrationTagOptions.some(option => option.value === tag)
  );
}
