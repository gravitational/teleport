import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import ResourceService, {
  KindAuthConnectors,
  makeResource,
  makeResourceList,
  Resource,
} from 'teleport/services/resources';

class ResourceServiceE extends ResourceService {
  fetchAuthConnectors() {
    return api
      .get(cfg.getAuthConnectorsListUrl())
      .then(res => makeResourceList<KindAuthConnectors>(res));
  }

  updateSamlConnector(name: string, content: string) {
    return api
      .put(cfg.getSamlConnectorsUrl(name), { content })
      .then(res => makeResource<'saml'>(res));
  }

  updateOidcConnector(name: string, content: string) {
    return api
      .put(cfg.getOidcConnectorsUrl(name), { content })
      .then(res => makeResource<'oidc'>(res));
  }

  createSamlConnector(content: string) {
    return api
      .post(cfg.getSamlConnectorsUrl(), { content })
      .then(res => makeResource<'saml'>(res));
  }

  createOidcConnector(content: string) {
    return api
      .post(cfg.getOidcConnectorsUrl(), { content })
      .then(res => makeResource<'oidc'>(res));
  }

  deleteSamlConnector(name: string) {
    return api.delete(cfg.getSamlConnectorsUrl(name));
  }

  deleteOidcConnector(name: string) {
    return api.delete(cfg.getOidcConnectorsUrl(name));
  }

  createConnector(
    kind: KindAuthConnectors,
    content: string
  ): Promise<Resource<KindAuthConnectors>> {
    switch (kind) {
      case 'oidc':
        return this.createOidcConnector(content);
      case 'saml':
        return this.createSamlConnector(content);
      default:
        return super.createGithubConnector(content);
    }
  }

  updateConnector(
    kind: KindAuthConnectors,
    name: string,
    content: string
  ): Promise<Resource<KindAuthConnectors>> {
    switch (kind) {
      case 'oidc':
        return this.updateOidcConnector(name, content);
      case 'saml':
        return this.updateSamlConnector(name, content);
      default:
        return super.updateGithubConnector(name, content);
    }
  }

  deleteConnector(kind: KindAuthConnectors, name: string) {
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
