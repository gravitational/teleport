import ResourceService, {
  makeResource,
  makeResourceList,
} from 'teleport/services/resources';
import { AuthProviderType } from 'shared/services';
import api from 'teleport/services/api';
import cfg from 'e-teleport/config';

class ResourceServiceE extends ResourceService {
  fetchAuthConnectors() {
    return api.get(cfg.getAuthConnectorsListUrl()).then(makeResourceList);
  }

  updateSamlConnector(content: string) {
    return api.put(cfg.getSamlConnectorsUrl(), { content }).then(makeResource);
  }

  updateOidcConnector(content: string) {
    return api.put(cfg.getOidcConnectorsUrl(), { content }).then(makeResource);
  }

  createSamlConnector(content: string) {
    return api.post(cfg.getSamlConnectorsUrl(), { content }).then(makeResource);
  }

  createOidcConnector(content: string) {
    return api.post(cfg.getOidcConnectorsUrl(), { content }).then(makeResource);
  }

  deleteSamlConnector(name: string) {
    return api.delete(cfg.getSamlConnectorsUrl(name));
  }

  deleteOidcConnector(name: string) {
    return api.delete(cfg.getOidcConnectorsUrl(name));
  }

  createConnector(kind: AuthProviderType, content: string) {
    switch (kind) {
      case 'oidc':
        return this.createOidcConnector(content);
      case 'saml':
        return this.createSamlConnector(content);
      default:
        return super.createGithubConnector(content);
    }
  }

  updateConnector(kind: AuthProviderType, content: string) {
    switch (kind) {
      case 'oidc':
        return this.updateOidcConnector(content);
      case 'saml':
        return this.updateSamlConnector(content);
      default:
        return super.updateGithubConnector(content);
    }
  }

  deleteConnector(kind: AuthProviderType, name: string) {
    switch (kind) {
      case 'oidc':
        return this.deleteOidcConnector(name);
      case 'saml':
        return this.deleteSamlConnector(name);
      default:
        return super.deleteGithubConnector(name);
    }
  }
}

export default ResourceServiceE;
