// EgressMode controls outbound network access from a beam.
type EgressMode = 'unrestricted' | 'restricted';

// Protocol is how a published beam application is exposed.
export type Protocol = 'http' | 'tcp';

// ComputeStatus reflects the lifecycle of the beam's underlying microVM.
export type ComputeStatus = '' | 'provision_pending' | 'provision_complete';

export type { Beam, BeamsListResponse } from './validation';

export type BeamsSortField = 'name' | 'alias' | 'user' | 'expires';

type BeamsSortDir = 'asc' | 'desc';

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
