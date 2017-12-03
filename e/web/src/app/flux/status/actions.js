import { makeStatus } from 'telebase-app/flux/status/actions'
import * as RT from './constants';

export const saveAuthProviderStatus = makeStatus(RT.TRYING_TO_SAVE_AUTH_PROVIDER);
export const saveClusterStatus = makeStatus(RT.TRYING_TO_SAVE_CLUSTER);
export const saveRoleStatus = makeStatus(RT.TRYING_TO_SAVE_ROLE);
export const deleteResourceStatus = makeStatus(RT.TRYING_TO_DELETE_RESOURCE);

