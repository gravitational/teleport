/**
 * RolesSelectedFor describes what purpose the roles were selected for:
 *   - eligibility: describes required roles to be eligible members/owners
 *   - grant: describes granting roles to members/owners
 */
export type RolesSelectedFor = 'eligibility' | 'grants';

export type UserKind = 'Members' | 'Owners';
