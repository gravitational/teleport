// Do not add new paths to this list, instead fix the underlying problem which causes console.error
// or console.warn to be used.
//
// If the test is expected to use either of those console functions, follow the advice from the
// error message.
module.exports = [
  'e/web/teleport/src/Account/Recovery/Recovery.test.tsx',
  'e/web/teleport/src/Billing/common/CancelAccountDialog.test.tsx',
  'e/web/teleport/src/Billing/common/StatusBanner.test.tsx',
  'e/web/teleport/src/Discover/SamlApplication/GcpWorkforce/ConfigureWorkforcePool/ConfigureWorkforcePool.test.tsx',
  'e/web/teleport/src/Integrations/IntegrationEnroll/ExternalAuditStorage/ConfigurePermissions/ConfigurePermissions.test.tsx',
  'e/web/teleport/src/Integrations/IntegrationEnroll/ExternalAuditStorage/ExternalAuditStorage.test.tsx',
  'e/web/teleport/src/Main/Main.test.tsx',
  'e/web/teleport/src/Workflow/NewRequest/NewRequest.test.tsx',
];
