// FormDataField defines the hardcoded form field
// names expected by the backend.
export enum FormDataField {
  AuthConnectorName = 'authConnectorName',
  DefaultOwners = 'defaultOwners',
  TenantId = 'tenantId',
  ClientId = 'clientId',
  AccessGraph = 'accessGraph',
  AccessGraphCache = 'accessGraphCache',
  GroupFilters = 'groupFilters',
}

/**
 * Filters defines resource filter fields.
 * Name matches with the proto PluginSyncFilter type.
 */
export type Filters = {
  id: string[];
  nameRegex: string[];
  excludeId: string[];
  excludeNameRegex: string[];
};
