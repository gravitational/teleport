import { requiredHttpsUrl } from './rules';

describe('requiredHttpsUrl', () => {
  test.each`
    url                                                | expected
    ${''}                                              | ${{ valid: false, message: 'SCIM endpoint is required' }}
    ${'http://example.com'}                            | ${{ valid: false, message: 'SCIM endpoint must be an HTTPS endpoint' }}
    ${'tcp://example.com'}                             | ${{ valid: false, message: 'SCIM endpoint must be an HTTPS endpoint' }}
    ${'javascript://com'}                              | ${{ valid: false, message: 'SCIM endpoint must be an HTTPS endpoint' }}
    ${'example.com'}                                   | ${{ valid: false, message: 'SCIM endpoint is invalid' }}
    ${'example'}                                       | ${{ valid: false, message: 'SCIM endpoint is invalid' }}
    ${'https://example.com'}                           | ${{ valid: true }}
    ${'https://subdomain.example.com'}                 | ${{ valid: true }}
    ${'https://subdomain.example.com/path?query=test'} | ${{ valid: true }}
  `('url: $url', ({ url, expected }) => {
    expect(requiredHttpsUrl(url)()).toEqual(expect.objectContaining(expected));
  });
});
