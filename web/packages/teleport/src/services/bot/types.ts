export enum BotJoinMethod {
  GitHub = 'github-actions',
}


export type GitHubBotConfig = {
  botName: string,
  rules: RepositoryRule[],
  labels: object,
}

type RepositoryRule = {
  repository: string
  repositoryOwner: string
  workflow?: string
  environment?: string
  actor?: string
  ref?: string
  refType?: 'branch' | 'tag'
}