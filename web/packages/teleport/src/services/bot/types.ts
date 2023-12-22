export enum BotJoinMethod {
  GitHub = 'github',
}

export type Bot = {
  name: string
  roles: string[]
}

export type GitHubBotConfig = Bot & {
  repository: string
  subject: string
  repositoryOwner: string
  workflow: string
  environment: string
  actor: string
  ref: string
  refType: 'branch' | 'tag'
}