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

import React, { useContext, useState } from 'react';

import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import { getErrorMessage } from 'shared/utils/error';

import { ResourceLabel } from 'teleport/services/agents';
import auth from 'teleport/services/auth';
import {
  createBotToken,
  GITHUB_ACTIONS_LABEL_KEY,
  createBot as serviceCreateBot,
} from 'teleport/services/bot';
import {
  BotUiFlow,
  CreateBotRequest,
  GitHubRepoRule,
} from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

import { GITHUB_HOST, parseRepoAddress, RefTypeOption } from '../Shared/github';

const GITHUB_ACTIONS_LABEL_VAL = BotUiFlow.GitHubActionsSsh;

type GitHubFlowContext = {
  attempt: Attempt;
  createBotRequest: CreateBotRequest;
  setCreateBotRequest: React.Dispatch<React.SetStateAction<CreateBotRequest>>;
  repoRules: Rule[];
  setRepoRules: React.Dispatch<React.SetStateAction<Rule[]>>;
  addEmptyRepoRule: () => void;
  tokenName: string;
  createBot: () => Promise<boolean>;
  resetAttempt: () => void;
};

const noop = () => {};
const gitHubFlowContext = React.createContext<GitHubFlowContext>({
  addEmptyRepoRule: noop,
  attempt: {
    status: '',
    statusText: undefined,
    statusCode: undefined,
  },
  createBotRequest: {
    botName: '',
    labels: [],
    roles: [],
    login: '',
  },
  setCreateBotRequest: noop,
  repoRules: [],
  setRepoRules: noop,
  tokenName: '',
  createBot: () => Promise.reject('noop'),
  resetAttempt: noop,
});

export const initialBotState = {
  labels: [{ name: '*', value: '*' }],
  login: '',
  botName: '',
  roles: [],
};

export function GitHubSshFlowProvider({
  children,
  bot = initialBotState,
}: React.PropsWithChildren<{ bot?: CreateBotRequest }>) {
  const { resourceService } = useTeleport();
  const { attempt, setAttempt } = useAttempt();
  const [createBotRequest, setCreateBotRequest] =
    useState<CreateBotRequest>(bot);
  const [repoRules, setRepoRules] = useState<Rule[]>([defaultRule]);
  const [tokenName, setTokenName] = useState('');

  function addEmptyRepoRule() {
    setRepoRules([...repoRules, defaultRule]);
  }

  function resetAttempt() {
    if (attempt.status !== 'processing') {
      setAttempt({ status: '' });
    }
  }

  // setting attempt to "success" is skipped because
  // it saves a re-render caused by it since
  // after the fetch is successful, we render the
  // finish step.
  async function createBot(): Promise<boolean> {
    setAttempt({ status: 'processing' });

    try {
      // Re-authn once, so we can re-use it for other actions
      // that require re-authn.
      const mfaResponse = await auth.getMfaChallengeResponseForAdminAction(
        true /* allow re-use */
      );

      await resourceService.createRole(
        getRoleYaml(
          createBotRequest.botName,
          createBotRequest.labels,
          createBotRequest.login
        ),
        mfaResponse
      );

      let repoHost = '';
      // Check if user sent a GitHub Enterprise host address.
      // We can just check the first rule, as the UI will not allow
      // using different hosts on multiple rules.
      if (repoRules.length > 0) {
        const { host } = parseRepoAddress(repoRules[0].repoAddress);
        // the enterprise server host should be omited if using github.com
        if (host !== GITHUB_HOST) {
          repoHost = host;
        }
      }

      const token = await createBotToken(
        {
          integrationName: createBotRequest.botName,
          joinMethod: 'github',
          webFlowLabel: GITHUB_ACTIONS_LABEL_VAL,
          gitHub: {
            enterpriseServerHost: repoHost,
            allow: repoRules.map((r): GitHubRepoRule => {
              const { owner, repository } = parseRepoAddress(r.repoAddress);
              return {
                repository: `${owner}/${repository}`,
                repositoryOwner: owner,
                actor: r.actor,
                environment: r.environment,
                ref: r.ref,
                refType: r.refType.value || undefined,
                workflow: r.workflowName,
              };
            }),
          },
        },
        mfaResponse
      );
      setTokenName(token.id);

      await serviceCreateBot(
        {
          ...createBotRequest,
          roles: [createBotRequest.botName],
        },
        mfaResponse
      );

      return true; // successful
    } catch (err) {
      setAttempt({ status: 'failed', statusText: getErrorMessage(err) });
      return false; // unsuccessful
    }
  }

  const value: GitHubFlowContext = {
    attempt,
    createBotRequest,
    setCreateBotRequest,
    repoRules,
    setRepoRules,
    addEmptyRepoRule,
    tokenName,
    createBot,
    resetAttempt,
  };

  return (
    <gitHubFlowContext.Provider value={value}>
      {children}
    </gitHubFlowContext.Provider>
  );
}

export function useGitHubSshFlow(): GitHubFlowContext {
  return useContext(gitHubFlowContext);
}

export type Rule = {
  workflowName: string;
  environment: string;
  ref: string;
  refType: RefTypeOption;
  repoAddress: string;
  actor: string;
};

export const defaultRule: Rule = {
  workflowName: '',
  environment: '',
  ref: '',
  refType: { label: 'any', value: '' },
  repoAddress: '',
  actor: '',
};

function getRoleYaml(
  botName: string,
  labels: ResourceLabel[],
  login: string
): string {
  const nodeLabels = labels
    .map(label => `'${label.name}': '${label.value}'`)
    .join('\n      ');

  return `kind: role
metadata:
  name: ${botName}
  labels:
    ${GITHUB_ACTIONS_LABEL_KEY}: ${GITHUB_ACTIONS_LABEL_VAL}
spec:
  allow:
    # List of allowed SSH logins
    logins: [${login}]

    # List of node labels that users can SSH into
    node_labels:
      ${nodeLabels}
    options:
      max_session_ttl: 8h0m0s
version: v7
  `;
}
