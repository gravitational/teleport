export type RenewSessionRequest = {
  requestId?: string;
  switchback?: boolean;
};

export type BearerToken = {
  accessToken: string;
  expiresIn: string;
  created: number;
  sessionExpires: Date;
  sessionInactiveTimeout: number;
};
