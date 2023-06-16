import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import {
  CreateSamlIdpServiceProviderRequest,
  CreateSamlIdpServiceProviderResponse,
} from './types';

export class IdpService {
  createSamlIdpServiceProvider(
    req: CreateSamlIdpServiceProviderRequest
  ): Promise<CreateSamlIdpServiceProviderResponse> {
    return api.post(cfg.api.samlIdpPath, req);
  }
}
