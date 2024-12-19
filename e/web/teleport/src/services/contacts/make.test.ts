import { makeContact } from './make';
import { ContactType } from './types';

describe('makeContact', () => {
  it('creates a verified contact with both business and security flags set', () => {
    const result = makeContact({
      name: 'John Doe',
      accountID: '12345',
      verifyToken: 'abc',
      email: 'john@example.com',
      contactType: ContactType.Business | ContactType.Security,
      contactState: 'CONTACT_STATE_ACTIVE',
    });
    expect(result.name).toBe('John Doe');
    expect(result.accountId).toBe('12345');
    expect(result.verifyToken).toBe('abc');
    expect(result.email).toBe('john@example.com');
    expect(result.contactType).toBe(
      ContactType.Business | ContactType.Security
    );
  });

  it('creates a pending contact with no flags set', () => {
    const result = makeContact({
      name: '',
      accountID: '67890',
      verifyToken: 'xyz',
      email: 'jane@example.com',
      contactType: 0,
      contactState: 'CONTACT_STATE_PENDING',
    });
    expect(result.name).toBe('');
    expect(result.accountId).toBe('67890');
    expect(result.verifyToken).toBe('xyz');
    expect(result.email).toBe('jane@example.com');
    expect(result.contactType).toBe(0);
  });

  it('creates an expired security contact', () => {
    const result = makeContact({
      contactState: 'CONTACT_STATE_EXPIRED',
      contactType: ContactType.Security,
    });
    expect(result.contactType).toBe(ContactType.Security);
  });
});
