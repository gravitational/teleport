export function generateOktaAdminBaseLink({
  orgUrl,
  appName,
  appId,
}: {
  orgUrl: string;
  appName: string;
  appId: string;
}) {
  const subdomain = orgUrl.replace(/.okta.com[/]?$/, '');
  return `${subdomain}-admin.okta.com/admin/app/${appName}/instance/${appId}`;
}

export function generateOktaSamlAppUrl(props: {
  orgUrl: string;
  appName: string;
  appId: string;
}) {
  const adminBaseUrl = generateOktaAdminBaseLink(props);

  return `${adminBaseUrl}/#tab-general`;
}

export function generateOktaScimSettingsUrl(props: {
  orgUrl: string;
  appName: string;
  appId: string;
}) {
  const adminBaseUrl = generateOktaAdminBaseLink(props);
  return `${adminBaseUrl}/#tab-user-management/api-credentials`;
}
