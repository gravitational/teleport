import { makeGetter } from 'telebase-app/flux/status/getters'
import * as RT from './constants';

export const saveAuthProviderAttempt = makeGetter(RT.TRYING_TO_SAVE_AUTH_PROVIDER);
export const saveClusterAttempt = makeGetter(RT.TRYING_TO_SAVE_CLUSTER);
export const saveRoleAttempt = makeGetter(RT.TRYING_TO_SAVE_ROLE);
export const deleteResourceAttempt = makeGetter(RT.TRYING_TO_DELETE_RESOURCE);
