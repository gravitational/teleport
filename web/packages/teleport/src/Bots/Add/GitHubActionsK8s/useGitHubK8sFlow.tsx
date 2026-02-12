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

import { useMutation } from '@tanstack/react-query';
import React, {
  PropsWithChildren,
  useContext,
  useEffect,
  useReducer,
} from 'react';
import { useDebounceCallback } from 'usehooks-ts';

import { generateGhaK8sTemplates } from 'teleport/services/bot/bot';
import { RefType } from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import { GITHUB_HOST, parseRepoAddress } from '../Shared/github';
import { KubernetesLabel } from '../Shared/kubernetes';
import { useTracking } from '../Shared/useTracking';
import { makeGhaWorkflow } from './templates';

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

  const ctx = useTeleport();
  const cluster = ctx.storeUser.state.cluster;

  const tracking = useTracking();

  // This effect sends a user event when the flow starts and ends
  useEffect(() => {
    tracking.start();
    return () => {
      tracking.complete();
    };
  }, [tracking]);

  const { mutate, data, isPending, error } = useMutation({
    mutationFn: (vars: Parameters<typeof generateGhaK8sTemplates>[0]) =>
      generateGhaK8sTemplates(vars),
    retry: 3,
    scope: {
      id: 'gha-k8s', // mutations in this scope are executed serially, in-order
    },
  });

  // Hold the previous data value so we always have something to display after
  // the initial fetch. When the mutation runs, data is set to null until the
  // new data arrives.
  const prevData = usePrevious(data);

  const regenerateTemplates = useDebounceCallback(mutate, 1000);

  // This effect triggers the code templates to be regenerated when state
  // changes. It's debounced to reduce the number of api calls.
  useEffect(() => {
    const includeKubernetes =
      state.kubernetesGroups.length > 0 ||
      state.kubernetesLabels.length > 0 ||
      state.kubernetesUsers.length > 0;

    regenerateTemplates({
      github: {
        allow: [
          {
            environment: state.environment,
            owner: state.info?.owner || 'gravitational',
            ref: state.ref,
            ref_type: state.refType,
            repository: state.info?.repository || 'teleport',
            workflow: state.workflow,
            // actor: state.actor - we don't allow actor to be specified
          },
        ],
        enterprise_server_host:
          state.info?.host === GITHUB_HOST ? undefined : state.info?.host,
        enterprise_slug: state.enterpriseSlug,
        static_jwks: state.enterpriseJwks,
      },
      kubernetes: includeKubernetes
        ? {
            groups: state.kubernetesGroups,
            labels: state.kubernetesLabels.reduce<Record<string, string[]>>(
              (acc, cur) => {
                const existing = acc[cur.name];
                if (existing) {
                  existing.push(...cur.values);
                } else {
                  acc[cur.name] = cur.values;
                }
                return acc;
              },
              {}
            ),
            users: state.kubernetesUsers,
          }
        : undefined,
    });
  }, [
    regenerateTemplates,
    state.enterpriseJwks,
    state.enterpriseSlug,
    state.environment,
    state.info?.host,
    state.info?.owner,
    state.info?.repository,
    state.kubernetesGroups,
    state.kubernetesLabels,
    state.kubernetesUsers,
    state.ref,
    state.refType,
    state.workflow,
  ]);

  const value = {
    state,
    dispatch,
    template: {
      terraform: {
        data: (data ?? prevData)?.terraform,
        loading: isPending,
        error,
      },
      ghaWorkflow: makeGhaWorkflow({
        tokenName: `gha-${state.info?.owner ?? 'gravitational'}-${state.info?.repository ?? 'teleport'}`,
        clusterPublicUrl: cluster.publicURL,
        clusterName: state.kubernetesCluster,
      }),
    },
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
      const branch = action.value;
      const ref = branch
        ? branch.startsWith('refs/heads/')
          ? branch
          : `refs/heads/${branch}`
        : '';
      return {
        ...prev,
        branch,
        allowAnyBranch: false,
        ref,
        refType: 'branch',
      };
    case 'allow-any-branch-toggled':
      return {
        ...prev,
        allowAnyBranch: action.value,
        branch: '',
        ref: '',
      };
    case 'ref-changed': {
      const branch =
        prev.refType === 'branch'
          ? action.value.replace(/^refs\/heads\//, '')
          : '';
      return {
        ...prev,
        ref: action.value,
        // Keep ref in sync with branch while ref type is 'branch'
        branch: branch,
        isBranchDisabled: prev.refType !== 'branch',
      };
    }
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
    case 'kubernetes-labels-changed':
      return {
        ...prev,
        kubernetesLabels: action.value,
      };
    case 'kubernetes-users-changed':
      return {
        ...prev,
        kubernetesUsers: action.value,
      };
    case 'kubernetes-cluster-changed':
      return {
        ...prev,
        kubernetesCluster: action.value,
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
        | 'jwks-changed'
        | 'kubernetes-cluster-changed';
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
    }
  | {
      type: 'kubernetes-labels-changed';
      value: KubernetesLabel[];
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
  kubernetesLabels: KubernetesLabel[];
  kubernetesUsers: string[];
  kubernetesCluster: string;
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
  kubernetesLabels: [{ name: '*', values: ['*'] }],
  kubernetesUsers: [],
  kubernetesCluster: '',
};

type Context = {
  state: State;
  dispatch: React.ActionDispatch<[Action]>;
  template: {
    terraform: {
      data?: string;
      loading: boolean;
      error: Error | null;
    };
    ghaWorkflow?: string;
  };
};
const context = React.createContext<Context>({
  dispatch: () => {
    throw new Error('not implemented');
  },
  state: defaultState,
  template: {
    terraform: {
      loading: false,
      error: null,
    },
  },
});

function usePrevious<T>(value: T) {
  const [current, setCurrent] = React.useState<T>(value);
  const [previous, setPrevious] = React.useState<T | undefined>(undefined);

  if (value !== current) {
    setPrevious(current);
    setCurrent(value);
  }

  return previous;
}
