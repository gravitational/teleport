export type Okta = {
  hasPlugin: boolean;
  hasAppGroupSyncEnabled: boolean;
  hasConfiguredOauthCredentials: boolean;
  emitEvent(): void;
};

export type UserCategory = 'owner' | 'member';
