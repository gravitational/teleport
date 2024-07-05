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

/**
 * SamlIdpServiceProvider defines fields that are available
 * in the SAMLIdPServiceProviderV1 proto in the backend.
 */
export type SamlIdpServiceProvider = {
  kind: string;
  metadata: {
    name: string;
    labels: Record<string, string>;
  };
  spec: SamlIdpServiceProviderSpec;
  version: string;
};

/**
 * SamlIdpServiceProviderSpec defines fields that are available
 * in the SAMLIdPServiceProviderSpecV1 proto in the backend.
 */
export type SamlIdpServiceProviderSpec = {
  acs_url: string;
  attribute_mapping: AttributeMapping[];
  entity_descriptor: string;
  entity_id: string;
  preset: string;
  relay_state: string;
};

/**
 * AttributeMapping defines SAML service provider
 * attribute mapping fields.
 */
export type AttributeMapping = {
  name: string;
  value: string;
  nameFormat?: string;
};

/**
 * SamlServiceProviderPreset defines SAML service provider preset types.
 * Used to define custom or pre-defined configuration flow.
 */
export enum SamlServiceProviderPreset {
  Unspecified = 'unspecified',
  Grafana = 'grafana',
  GcpWorkforce = 'gcp-workforce',
}
