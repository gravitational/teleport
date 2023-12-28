import React, { useState, useContext } from 'react';
import { Option } from 'shared/components/Select';

import useAttempt, { Attempt } from 'shared/hooks/useAttemptNext';
import { ResourceLabel } from 'teleport/services/agents';
import useTeleport from 'teleport/useTeleport';

type GitHubFlowContext = {
  attempt: Attempt;
  botConfig: BotConfig,
  setBotConfig: React.Dispatch<React.SetStateAction<BotConfig>>,
  repoRules: Rule[],
  setRepoRules: React.Dispatch<React.SetStateAction<Rule[]>>,
  addEmptyRepoRule: () => void,
  tokenName: string,
}

const stepsContext = React.createContext<GitHubFlowContext>(null);

export function GitHubFlowProvider({
  children,
}: React.PropsWithChildren) {
  const { botService } = useTeleport();
  const { attempt, run } = useAttempt();
  const [botConfig, setBotConfig] = useState<BotConfig>({ labels: [{ name: '*', value: '*' }], login: '', name: '' })
  const [repoRules, setRepoRules] = useState<Rule[]>([defaultRule])
  const [tokenName] = useState('')

  function addEmptyRepoRule() {
    setRepoRules([...repoRules, defaultRule])
  }


  const value: GitHubFlowContext = {
    attempt,
    botConfig,
    setBotConfig,
    repoRules,
    setRepoRules,
    addEmptyRepoRule,
    tokenName,
  };

  return (
    <stepsContext.Provider value={value}>{children}</stepsContext.Provider>
  );
}

export function useGitHubFlow(): GitHubFlowContext {
  return useContext(stepsContext);
}

export type BotConfig = {
  labels: ResourceLabel[],
  login: string,
  name: string,
}

export type RefType = 'branch' | 'tag'

export type RefTypeOption = Option<RefType>;

export type Rule = {
  workflowName: string,
  environment: string,
  ref: string,
  refType: RefTypeOption,
  repoAddress: string,
  actor: string,
}

export const defaultRule: Rule = {
  workflowName: '',
  environment: '',
  ref: '',
  refType: { label: 'Branch', value: 'branch' },
  repoAddress: '',
  actor: '',
}
