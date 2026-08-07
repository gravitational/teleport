import { parseClientIpRestrictionResponse } from './validation';

describe('parseClientIpRestrictionResponse', () => {
  test('passes a fully populated resource through', () => {
    expect(
      parseClientIpRestrictionResponse({
        cidrs: ['10.0.0.0/8'],
        mode: 'enforced',
        expires: '2026-08-07T12:00:00Z',
        status: 'active',
        revision: 'rev-1',
      })
    ).toEqual({
      cidrs: ['10.0.0.0/8'],
      mode: 'enforced',
      expires: '2026-08-07T12:00:00Z',
      status: 'active',
      revision: 'rev-1',
    });
  });

  test('fills in the fields the endpoint omits when unset', () => {
    // Everything but cidrs is `omitempty` on the Go side, so a draft with no
    // deadline arrives without an `expires` key at all.
    expect(
      parseClientIpRestrictionResponse({
        cidrs: ['10.0.0.0/8'],
        mode: 'draft',
        status: 'draft',
      })
    ).toEqual({
      cidrs: ['10.0.0.0/8'],
      mode: 'draft',
      status: 'draft',
      revision: '',
    });
  });

  test('turns what a tenant with no allowlist returns into an empty resource', () => {
    // A nil slice marshals to null, and the rest of the zero resource is omitted.
    expect(parseClientIpRestrictionResponse({ cidrs: null })).toEqual({
      cidrs: [],
      mode: '',
      status: '',
      revision: '',
    });
  });

  test('throws on a response of the wrong shape', () => {
    expect(() =>
      parseClientIpRestrictionResponse({ cidrs: 'not-a-list' })
    ).toThrow(/failed to parse/i);
    expect(() => parseClientIpRestrictionResponse(undefined)).toThrow(
      /failed to parse/i
    );
  });
});
