/**
 * ContactType is a bit field whose bits represents the type of a contact.
 * It should be kept in sync with `ContactType` field in
 * `cloud/contact.go` in the Cloud repository.
 */
export enum ContactType {
  Business = 1,
  Security = 2,
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
  verified: boolean;
  verifyExpiresAt: number;
};
