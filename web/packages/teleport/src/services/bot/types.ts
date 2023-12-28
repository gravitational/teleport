export type BotConfig = {
  botName: string,
  labels: object,
  roles: string[],
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