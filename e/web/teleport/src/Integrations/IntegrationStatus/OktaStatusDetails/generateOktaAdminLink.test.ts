import {
  generateOktaAdminBaseLink,
  generateOktaSamlAppUrl,
  generateOktaScimSettingsUrl,
} from './generateOktaAdminLink';

test('generateOktaAdminBaseLink', () => {
  expect(
    generateOktaAdminBaseLink({
      orgUrl: 'https://llama.okta.com',
      appId: 'app-id',
      appName: 'app-name',
    })
  ).toBe('https://llama-admin.okta.com/admin/app/app-name/instance/app-id');
});

test('generateOktaSamlAppUrl', () => {
  expect(
    generateOktaSamlAppUrl({
      orgUrl: 'https://llama.okta.com',
      appId: 'app-id',
      appName: 'app-name',
    })
  ).toBe(
    'https://llama-admin.okta.com/admin/app/app-name/instance/app-id/#tab-general'
  );
});

test('generateOktaScimSettingsUrl', () => {
  expect(
    generateOktaScimSettingsUrl({
      orgUrl: 'https://llama.okta.com',
      appId: 'app-id',
      appName: 'app-name',
    })
  ).toBe(
    'https://llama-admin.okta.com/admin/app/app-name/instance/app-id/#tab-user-management/api-credentials'
  );
});
