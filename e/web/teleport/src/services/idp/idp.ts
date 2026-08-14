import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import type { SamlIdpServiceProvider } from 'teleport/services/samlidp/types';

import {
  CreateSamlIdpServiceProviderRequest,
  SamlIdpMetadataResponse,
} from './types';

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
  deleteSamlIdpServiceProvider(name: string): Promise<void> {
    return api.delete(cfg.api.samlIdpPath + `/${name}`);
  }
  getIdPMetadataValues(): Promise<SamlIdpMetadataResponse> {
    return api.get(cfg.api.samlIdPMetadataValuesPath);
  }

  /**
   * getMetadataXml uses native fetch function to fetch an XML file.
   * Native fetch function is used because the api.get function expects
   * response to be a JSON data.
   * @returns {string} A string containing SAML IdP entity descriptor (XML file).
   */
  async getMetadataXml(): Promise<string> {
    const resp = await api.fetch(cfg.api.samlIdPMetadataFilePath);
    if (resp.status !== 200) {
      throw new Error('invalid response');
    }
    const metadata = await resp.text();
    return metadata;
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
