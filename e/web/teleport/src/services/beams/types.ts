// EgressMode controls outbound network access from a beam.
export type EgressMode = 'unrestricted' | 'restricted';

// Protocol is how a published beam application is exposed.
export type Protocol = 'http' | 'tcp';

// ComputeStatus reflects the lifecycle of the beam's underlying microVM.
export type ComputeStatus = '' | 'provision_pending' | 'provision_complete';

// BeamPublish describes how a beam's app is exposed to Teleport users.
//
// `port` and `protocol` are read-only in the UI today
export type BeamPublish = {
  port: number;
  protocol: Protocol;
};

export type { Beam, BeamsListResponse } from './validation';

export type BeamsSortField = 'name' | 'alias' | 'user' | 'expires';

export type BeamsSortDir = 'asc' | 'desc';

export type BeamsListParams = {
  pageToken?: string;
  pageSize?: number;
  users?: string[];
  sortField?: BeamsSortField;
  sortDir?: BeamsSortDir;
};

export type CreateBeamRequest = {
  egress_mode?: EgressMode;
  allowed_domains?: string[];
};
