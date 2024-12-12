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

import React from 'react';

import { AuthConnectorTile } from '../AuthConnectorTile';
import getSsoIcon from '../ssoIcons/getSsoIcon';
import { AuthConnectorsGrid } from '../styles/ConnectorBox.styles';

export default function CTAConnectors() {
  return (
    <AuthConnectorsGrid>
      <AuthConnectorTile
        key={'oidc-cta'}
        kind="oidc"
        id={'oidc-cta'}
        name={'OIDC Connector'}
        Icon={getSsoIcon('oidc')}
        isDefault={false}
        isPlaceholder={false}
        onSetup={() => {}}
        isEnabled={false}
        isCTA={true}
        onEdit={() => {}}
        onDelete={() => {}}
        customDesc="Google, GitLab, Amazon and more"
      />
      <AuthConnectorTile
        key={'saml-cta'}
        kind="saml"
        id={'saml-cta'}
        name={'SAML Connector'}
        Icon={getSsoIcon('saml')}
        isDefault={false}
        isPlaceholder={false}
        onSetup={() => {}}
        isEnabled={false}
        isCTA={true}
        onEdit={() => {}}
        onDelete={() => {}}
        customDesc="Okta, OneLogin, Azure Active Directory, etc."
      />
    </AuthConnectorsGrid>
  );
}
