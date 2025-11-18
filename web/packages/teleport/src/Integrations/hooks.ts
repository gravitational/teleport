/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

import type {
  UseMutationOptions,
  UseMutationResult,
  UseQueryOptions,
  UseQueryResult,
} from '@tanstack/react-query';

import {
  IKindToType,
  IntegrationCreateRequest,
  IntegrationCreateResult,
  integrationService,
  IntegrationUpdateRequest,
  IntegrationUpdateResult,
  TypedIntegrationKind,
} from 'teleport/services/integrations';
import {
  createMutationHook,
  createQueryHook,
} from 'teleport/services/queryHelpers';

type FetchIntegrationsResponse = Awaited<
  ReturnType<typeof integrationService.fetchIntegrations>
>;

const { useQuery: _useFetchIntegrations, queryKey: FetchIntegrationsQueryKey } =
  createQueryHook<FetchIntegrationsResponse>(
    ['integrations', 'fetch'],
    integrationService.fetchIntegrations
  );

const {
  useQuery: _useFetchIntegration,
  createQueryKey: createFetchIntegrationQueryKey,
} = createQueryHook(
  ['integration', 'fetch'],
  integrationService.fetchIntegration
);

const {
  useMutation: _useCreateIntegration,
  createMutationKey: createCreateIntegrationMutationKey,
} = createMutationHook(
  ['integration', 'create'],
  integrationService.createIntegration
);

type UpdateIntegrationVars<K extends TypedIntegrationKind> = {
  name: string;
  req: IntegrationUpdateRequest<K>;
};

function updateIntegrationWithVars<K extends TypedIntegrationKind>(
  vars: UpdateIntegrationVars<K>
): Promise<IntegrationUpdateResult<IntegrationUpdateRequest<K>>> {
  return integrationService.updateIntegration(vars.name, vars.req);
}

const {
  useMutation: _useUpdateIntegration,
  createMutationKey: createUpdateIntegrationMutationKey,
} = createMutationHook(['integration', 'update'], updateIntegrationWithVars);

export {
  FetchIntegrationsQueryKey,
  createFetchIntegrationQueryKey,
  createCreateIntegrationMutationKey,
  createUpdateIntegrationMutationKey,
};

export function useFetchIntegrations(
  options?: Omit<
    UseQueryOptions<FetchIntegrationsResponse>,
    'queryKey' | 'queryFn'
  >
) {
  return _useFetchIntegrations(undefined, options);
}

export function useFetchIntegration<K extends keyof IKindToType>(
  name: string,
  options?: Omit<UseQueryOptions<IKindToType[K]>, 'queryKey' | 'queryFn'>
) {
  return _useFetchIntegration(name, options) as UseQueryResult<
    IKindToType[K],
    Error
  >;
}

export function useCreateIntegration<
  K extends TypedIntegrationKind,
  T extends IntegrationCreateRequest<K> = IntegrationCreateRequest<K>,
  R extends IntegrationCreateResult<T> = IntegrationCreateResult<T>,
>(
  options?: Omit<UseMutationOptions<R, Error, T>, 'mutationKey' | 'mutationFn'>
) {
  return _useCreateIntegration(options) as UseMutationResult<R, Error, T>;
}

export function useUpdateIntegration<
  K extends TypedIntegrationKind,
  T extends IntegrationUpdateRequest<K> = IntegrationUpdateRequest<K>,
  R extends IntegrationUpdateResult<T> = IntegrationUpdateResult<T>,
  V extends UpdateIntegrationVars<K> = UpdateIntegrationVars<K>,
>(
  options?: Omit<UseMutationOptions<R, Error, V>, 'mutationKey' | 'mutationFn'>
) {
  return _useUpdateIntegration(options) as UseMutationResult<R, Error, V>;
}
