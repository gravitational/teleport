/**
 * Each condition type like "StandardRoleConditions" and "AwsIcRoleConditions"
 * refers to separate role resources. So if BOTH conditions are defined and
 * in the guide editor user is:
 *  - creating access: 2 separate roles are created by backend.
 *  - updating access: 2 separate roles are updated by backend.
 */
export * from './awsic';
export * from './standard';
