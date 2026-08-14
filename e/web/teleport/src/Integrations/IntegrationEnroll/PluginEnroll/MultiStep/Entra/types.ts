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
  AccessListOwnersSource = 'accessListOwnersSource',
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

/**
 * SettingsType defines plugin configuration
 * type related to the Entra ID plugin UI.
 */
export enum SettingsType {
  GroupImport = 'groups-import',
}

/**
 * AccessListOwnersSource defines friendly source
 * names of the Access List owner. Value matches with
 * proto EntraIDAccessListOwnersSource type.
 */
export enum AccessListOwnersSource {
  Plugin = 'ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN',
  EntraId = 'ENTRAID_ACCESS_LIST_OWNERS_SOURCE_ENTRAID',
  PluginAndEntraId = 'ENTRAID_ACCESS_LIST_OWNERS_SOURCE_PLUGIN_AND_ENTRAID',
}

export function toFrienldyAccessListOwnersSource(source: string) {
  switch (source) {
    case AccessListOwnersSource.Plugin:
      return 'Plugin';
    case AccessListOwnersSource.EntraId:
      return 'Microsoft Entra ID';
    case AccessListOwnersSource.PluginAndEntraId:
      return 'Plugin and Microsoft Entra ID';
    default:
      /**
       * Backend may have introduced a new source type which is
       * unknown to the UI.
       */
      return 'Unknown';
  }
}
