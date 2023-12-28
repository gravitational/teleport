import { ResourceLabel } from '../agents';

export type BotConfig = {
  botName: string;
  labels: ResourceLabel[];
  roles: string[];
  login: string;
};
