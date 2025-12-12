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

import React, { PropsWithChildren, useContext, useReducer } from 'react';

import { RefType } from 'teleport/services/bot/types';

import { parseRepoAddress } from '../Shared/github';

export function useGitHubK8sFlow() {
  return useContext(context);
}

export function GitHubK8sFlowProvider(
  props: PropsWithChildren & {
    intitialState?: Partial<State>;
  }
) {
  const { children, intitialState = {} } = props;

  const [state, dispatch] = useReducer(reducer, {
    ...defaultState,
    ...intitialState,
  });

  const value = {
    state,
    dispatch,
  };

  return <context.Provider value={value}>{children}</context.Provider>;
}

function reducer(prev: State, action: Action): State {
  switch (action.type) {
    case 'github-url-changed':
      let info: ReturnType<typeof parseRepoAddress> | undefined = undefined;
      try {
        info = parseRepoAddress(action.value);
      } catch {
        /* Best endeavours */
      }

      return {
        ...prev,
        gitHubUrl: action.value,
        info,
      };
    case 'branch-changed':
      return {
        ...prev,
        branch: action.value,
        allowAnyBranch: false,
        ref: action.value,
        refType: 'branch',
      };
    case 'allow-any-branch-toggled':
      return {
        ...prev,
        allowAnyBranch: action.value,
        branch: '',
        ref: '',
      };
    case 'ref-changed':
      return {
        ...prev,
        ref: action.value,
        // Keep ref in sync with branch while ref type is 'branch'
        branch: prev.refType === 'branch' ? action.value : '',
        isBranchDisabled: prev.refType !== 'branch',
      };
    case 'ref-type-changed':
      return {
        ...prev,
        // Keep ref in sync with branch while ref type is 'branch'
        branch: action.value === 'branch' ? prev.ref : '',
        isBranchDisabled: action.value !== 'branch',
        refType: action.value,
      };
    case 'workflow-changed':
      return {
        ...prev,
        workflow: action.value,
      };
    case 'environment-changed':
      return {
        ...prev,
        environment: action.value,
      };
    case 'slug-changed':
      return {
        ...prev,
        enterpriseSlug: action.value,
      };
    case 'jwks-changed':
      return {
        ...prev,
        enterpriseJwks: action.value,
      };
    case 'kubernetes-groups-changed':
      return {
        ...prev,
        kubernetesGroups: action.value,
      };
    case 'kubernetes-users-changed':
      return {
        ...prev,
        kubernetesUsers: action.value,
      };
    default:
      const exhaustiveCheck: never = action;
      throw new Error(`Unhandled action type: ${exhaustiveCheck}`);
  }
}

type Action =
  | {
      type:
        | 'github-url-changed'
        | 'branch-changed'
        | 'ref-changed'
        | 'workflow-changed'
        | 'environment-changed'
        | 'slug-changed'
        | 'jwks-changed';
      value: string;
    }
  | {
      type: 'allow-any-branch-toggled';
      value: boolean;
    }
  | {
      type: 'ref-type-changed';
      value: RefType | '';
    }
  | {
      type: 'kubernetes-groups-changed' | 'kubernetes-users-changed';
      value: string[];
    };

type State = {
  gitHubUrl: string;
  info?: {
    host?: string;
    owner?: string;
    repository?: string;
  };
  branch: string;
  allowAnyBranch: boolean;
  isBranchDisabled: boolean;
  ref: string;
  refType: RefType | '';
  workflow: string;
  environment: string;
  enterpriseSlug: string;
  enterpriseJwks: string;
  kubernetesGroups: string[];
  kubernetesUsers: string[];
};

const defaultState: State = {
  gitHubUrl: '',
  branch: '',
  allowAnyBranch: false,
  isBranchDisabled: false,
  ref: '',
  refType: 'branch',
  workflow: '',
  environment: '',
  enterpriseSlug: '',
  enterpriseJwks: '',
  kubernetesGroups: [],
  kubernetesUsers: [],
};

type Context = {
  state: State;
  dispatch: React.ActionDispatch<[Action]>;
};
const context = React.createContext<Context>({
  dispatch: () => {
    throw new Error('not implemented');
  },
  state: defaultState,
});
