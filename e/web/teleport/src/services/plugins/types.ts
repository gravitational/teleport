export type Plugin = {
  name: string;
  details: string;
  status: PluginStatus;
  type: string;
};

export type PluginStatus = {
  code: string;
  description?: string;
};
