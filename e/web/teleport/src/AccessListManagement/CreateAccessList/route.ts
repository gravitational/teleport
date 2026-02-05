import cfg from 'e-teleport/config';

export function goToCreateAccessListFromOktaRoute(oktaOrgUrl: string) {
  return {
    pathname: cfg.routes.accessListNew,
    state: { oktaOrgUrl },
  };
}
