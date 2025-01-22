import cfg from 'e-teleport/config';
import api from 'teleport/services/api';
import auth from 'teleport/services/auth/auth';
import ResourceService, {
  DefaultAuthConnector,
  KindAuthConnectors,
  makeResource,
  makeResourceList,
  Resource,
} from 'teleport/services/resources';

class ResourceServiceE extends ResourceService {
  // TODO(rudream): Look into combining this method with the one from the OSS ResourceService and adding support for generics.
  async fetchAuthConnectors(): Promise<{
    defaultConnector: DefaultAuthConnector;
    connectors: Resource<KindAuthConnectors>[];
  }> {
    // MFA reuse needs to be allowed in case we need to fallback to another default connector
    const challengeResponse =
      await await auth.getMfaChallengeResponseForAdminAction(true);

    return api
      .get(cfg.getAuthConnectorsListUrl(), undefined, challengeResponse)
      .then(res => ({
        defaultConnector: {
          name: res.defaultConnectorName,
          type: res.defaultConnectorType,
        },
        connectors: makeResourceList<KindAuthConnectors>(res.connectors),
      }));
  }

  fetchSamlConnector(name: string) {
    return api
      .get(cfg.getSamlConnectorSpecificUrl(name))
      .then(res => makeResource<'saml'>(res));
  }

  updateSamlConnector(name: string, content: string) {
    return api
      .put(cfg.getSamlConnectorsUrl(name), { content })
      .then(res => makeResource<'saml'>(res));
  }

  fetchOidcConnector(name: string) {
    return api
      .get(cfg.getOidcConnectorSpecificUrl(name))
      .then(res => makeResource<'oidc'>(res));
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

  fetchConnector(kind: KindAuthConnectors, name: string) {
    switch (kind) {
      case 'oidc':
        return this.fetchOidcConnector(name);
      case 'saml':
        return this.fetchSamlConnector(name);
      default:
        return super.fetchGithubConnector(name);
    }
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
