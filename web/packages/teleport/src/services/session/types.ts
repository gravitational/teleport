export type RenewSessionRequest = {
  requestId?: string;
  switchback?: boolean;
};

export type BearerToken = {
  accessToken: string;
  expiresIn: string;
  created: number;
};

export type Session = {
  token: BearerToken;
  expires: Date;
};
