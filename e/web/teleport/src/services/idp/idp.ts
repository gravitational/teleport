import api from 'teleport/services/api';

import cfg from 'e-teleport/config';

import {
  CreateSamlIdpServiceProviderRequest,
  SAMLIdPMetadataResponse,
} from './types';

import type { SamlIdpServiceProvider } from 'teleport/services/samlidp/types';

export class IdpService {
  getSamlIdpServiceProvider(name: string): Promise<SamlIdpServiceProvider> {
    return api.get(cfg.api.samlIdpPath + `/${name}`);
  }
  createSamlIdpServiceProvider(
    req: CreateSamlIdpServiceProviderRequest
  ): Promise<SamlIdpServiceProvider> {
    return api.post(cfg.api.samlIdpPath, req);
  }
  updateSamlIdpServiceProvider(
    req: CreateSamlIdpServiceProviderRequest
  ): Promise<SamlIdpServiceProvider> {
    return api.put(cfg.api.samlIdpPath + `/${req.name}`, req);
  }
  getIdPMetadataValues(): Promise<SAMLIdPMetadataResponse> {
    return api.get(cfg.api.samlIdPMetadataValuesPath);
  }

  upsertRequest(
    spRequest: CreateSamlIdpServiceProviderRequest,
    isUpdateFlow: boolean
  ) {
    if (isUpdateFlow) {
      return this.updateSamlIdpServiceProvider(spRequest);
    } else {
      return this.createSamlIdpServiceProvider(spRequest);
    }
  }
}
