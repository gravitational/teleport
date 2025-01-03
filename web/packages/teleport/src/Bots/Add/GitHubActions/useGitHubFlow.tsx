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

import { Option } from 'shared/components/Select';
import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';

import { ResourceLabel } from 'teleport/services/agents';
import {
  createBotToken,
  GITHUB_ACTIONS_LABEL_KEY,
  createBot as serviceCreateBot,
} from 'teleport/services/bot';
import {
  BotUiFlow,
  CreateBotRequest,
  GitHubRepoRule,
  RefType,
} from 'teleport/services/bot/types';
import useTeleport from 'teleport/useTeleport';

export const GITHUB_HOST = 'github.com';
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

const gitHubFlowContext = React.createContext<GitHubFlowContext>(null);

export const initialBotState = {
  labels: [{ name: '*', value: '*' }],
  login: '',
  botName: '',
  roles: [],
};

export function GitHubFlowProvider({
  children,
  bot = initialBotState,
}: React.PropsWithChildren<{ bot?: CreateBotRequest }>) {
  const { resourceService } = useTeleport();
  const { attempt, run, setAttempt } = useAttempt();
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

  function createBot(): Promise<boolean> {
    return run(() =>
      resourceService
        .createRole(
          getRoleYaml(
            createBotRequest.botName,
            createBotRequest.labels,
            createBotRequest.login
          )
        )
        .then(() => {
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

          return createBotToken({
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
                  refType: r.refType.value || null,
                  workflow: r.workflowName,
                };
              }),
            },
          }).then(token => {
            setTokenName(token.id);
            return serviceCreateBot({
              ...createBotRequest,
              roles: [createBotRequest.botName],
            });
          });
        })
    );
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

export function useGitHubFlow(): GitHubFlowContext {
  return useContext(gitHubFlowContext);
}

export type RefTypeOption = Option<RefType | ''>;

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

/**
 * Parses the GitHub repository URL and returns the repository name and
 * its owner's name. Throws errors if parsing the URL fails or
 * the URL doesn't contains the expected format.
 * @param repoAddr repository address (with or without protocl)
 * @returns owner and repository name
 */
export function parseRepoAddress(repoAddr: string): {
  host: string;
  owner: string;
  repository: string;
} {
  // add protocol if it is missing
  if (!repoAddr.startsWith('http://') && !repoAddr.startsWith('https://')) {
    repoAddr = `https://${repoAddr}`;
  }

  let url;
  try {
    url = new URL(repoAddr);
  } catch {
    throw new Error('Must be a valid URL');
  }

  const paths = url.pathname.split('/');
  // expected length is 3, since pathname starts with a /, so paths[0] should be empty
  if (paths.length < 3) {
    throw new Error(
      'URL expected to be in the format https://<host>/<owner>/<repository>'
    );
  }

  const owner = paths[1];
  const repository = paths[2];
  if (owner.trim() === '' || repository.trim() == '') {
    throw new Error(
      'URL expected to be in the format https://<host>/<owner>/<repository>'
    );
  }

  return {
    host: url.host,
    owner,
    repository,
  };
}

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
