/**
 * ContactType is a bit field whose bits represents the type of a contact.
 * Each value is the bit in a bit field that represents the type of a contact.
 * It should be kept in sync with `ContactType` field in
 * `cloud/contact.go` in the Cloud repository.
 */
export enum ContactType {
  Business = 1 << 0,
  Security = 1 << 1,
}

// State of a contact in the verification process
export enum ContactVerification {
  Verified,
  Pending,
  Expired,
}

/**
 * Contact represents a Business and/or Security contact.
 */
export type Contact = {
  name: string;
  accountId: string;
  verifyToken: string;
  email: string;
  contactType: ContactType;
  verification: ContactVerification;
};
