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

import React, { createContext, useContext } from 'react';

import { Attempt } from 'shared/hooks/useAsync';

import type { ResourceSpec } from 'teleport/Discover/SelectResource/types';
import { SamlMeta } from 'teleport/Discover/useDiscover';
import type { SamlAppToDelete } from 'teleport/services/samlidp/types';
import type { Access } from 'teleport/services/user';

/**
 * SamlAppAction defines Saml application edit and delete actions.
 */
export interface SamlAppAction {
  /**
   * actions controls Saml menu button view and edit and delete onClick behaviour.
   */
  actions: {
    /**
     * showActions dictates whether to show or hide the Saml menu button.
     */
    showActions: boolean;
    /**
     * startEdit triggers Saml app edit flow.
     */
    startEdit: (resourceSpec: ResourceSpec) => void;
    /**
     * startDelete triggers Saml app delete flow.
     */
    startDelete: (resourceSpec: ResourceSpec) => void;
  };
  /**
   * currentAction specifies edit or delete mode.
   */
  currentAction?: SamlAppActionMode;
  /**
   * deleteSamlAppAttempt is an attempt to delete Saml
   * app in the backend.
   */
  deleteSamlAppAttempt?: Attempt<void>;
  /**
   * samlAppToDelete defines Saml app item that is to be
   * deleted from the unified view.
   */
  samlAppToDelete?: SamlAppToDelete;
  /**
   * fetchSamlResourceAttempt is an attempt to fetch
   * Saml resource spec from the backend. It is used to
   * pre-populate input fields in the Saml Discover flow.
   */
  fetchSamlResourceAttempt?: Attempt<SamlMeta>;
  /**
   * resourceSpec holds current Saml app resource spec.
   */
  resourceSpec?: ResourceSpec;
  /**
   * userSamlIdPPerm holds user's RBAC permissions to
   * saml_idp_service_provider resource.
   */
  userSamlIdPPerm?: Access;
  /**
   * clearAction clears edit or delete flow.
   */
  clearAction?: () => void;
  /**
   * onDelete handles Saml app delete in the backend.
   */
  onDelete?: () => void;
}

export const SamlAppActionContext = createContext<SamlAppAction>(null);

export function useSamlAppAction() {
  return useContext(SamlAppActionContext);
}

/**
 * SamlAppActionProvider is a dummy provider to satisfy
 * SamlAppActionContext in Teleport community edition.
 */
export function SamlAppActionProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const value: SamlAppAction = {
    actions: {
      showActions: false,
      startEdit: null,
      startDelete: null,
    },
  };

  return (
    <SamlAppActionContext.Provider value={value}>
      {children}
    </SamlAppActionContext.Provider>
  );
}

export type SamlAppActionMode = 'edit' | 'delete';
