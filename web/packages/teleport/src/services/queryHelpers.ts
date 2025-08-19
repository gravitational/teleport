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

import * as reactQuery from '@tanstack/react-query';
import {
  type DataTag,
  type DefaultError,
  type InfiniteData,
  type QueryFunction,
  type UseInfiniteQueryOptions,
  type UseInfiniteQueryResult,
  type UseQueryOptions,
  type UseQueryResult,
  type UseSuspenseInfiniteQueryOptions,
  type UseSuspenseInfiniteQueryResult,
  type UseSuspenseQueryOptions,
  type UseSuspenseQueryResult,
} from '@tanstack/react-query';

export type QueryHook<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
> = (
  variables?: TVariables,
  options?: Omit<UseQueryOptions<TData, TError>, 'queryKey' | 'queryFn'>
) => UseQueryResult<TData, TError>;

export type SuspenseQueryHook<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
> = (
  variables?: TVariables,
  options?: Omit<UseSuspenseQueryOptions<TData, TError>, 'queryKey' | 'queryFn'>
) => UseSuspenseQueryResult<TData, TError>;

export type InfiniteQueryHook<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
  TPageParam = string,
> = (
  variables: TVariables,
  options: Omit<
    UseInfiniteQueryOptions<
      TData,
      TError,
      InfiniteData<TData, TPageParam>,
      DataTag<string[], TData, TError>,
      TPageParam
    >,
    'queryKey' | 'queryFn'
  >
) => UseInfiniteQueryResult<InfiniteData<TData, TPageParam>, TError>;

export type SuspenseInfiniteQueryHook<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
  TPageParam = string,
> = (
  variables: TVariables,
  options: Omit<
    UseSuspenseInfiniteQueryOptions<
      TData,
      TError,
      InfiniteData<TData, TPageParam>,
      DataTag<string[], TData, TError>,
      TPageParam
    >,
    'queryKey' | 'queryFn'
  >
) => UseSuspenseInfiniteQueryResult<InfiniteData<TData, TPageParam>, TError>;

export interface WrappedQuery<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
  TPageParam = string,
> {
  createQueryKey: (variables?: TVariables) => DataTag<string[], TData, TError>;
  queryKey: DataTag<string[], TData, TError>;
  queryFn: (variables: TVariables) => QueryFunction<TData>;
  useQuery: QueryHook<TData, TVariables, TError>;
  createQuery: (variables?: TVariables) => {
    queryKey: DataTag<string[], TData, TError>;
    queryFn: QueryFunction<TData>;
  };
  useSuspenseQuery: SuspenseQueryHook<TData, TVariables, TError>;
  useInfiniteQuery: InfiniteQueryHook<TData, TVariables, TError, TPageParam>;
  useSuspenseInfiniteQuery: SuspenseInfiniteQueryHook<
    TData,
    TVariables,
    TError,
    TPageParam
  >;
}

type SignalOnlyQueryFn<TData> = (signal: AbortSignal) => Promise<TData>;
type VariablesQueryFn<TData, TVariables> = (
  variables: TVariables,
  signal: AbortSignal
) => Promise<TData>;

type QueryFn<TData, TVariables> = TVariables extends void
  ? SignalOnlyQueryFn<TData>
  : VariablesQueryFn<TData, TVariables>;

/**
 * createQueryHook is a utility function to create a TanStack Query `useQuery`
 * hook from a service method.
 *
 * This is useful for quickly creating a query, with the variables passed through
 * to the service method automatically added to the query key.
 *
 * This function creates both the wrapped `useQuery` hook, a method to generate
 * the query key from a given set of variables, and the query key itself.
 *
 * The generated query key is tagged with the data type and error type, so that
 * when using the query key to manually change query data, the type of the
 * data is automatically inferred.
 *
 * This is useful for being able to update the results of a query when needed
 * (e.g. during mutations), without having to recreate the query key and have
 * it defined in multiple places.
 *
 * This function expects the service method to be a function that looks like either:
 *   (signal?: AbortSignal) => Promise<TData>
 *     or
 *   (variables: TVariables, signal?: AbortSignal) => Promise<TData>
 *
 * If you are using this function with a service method that takes multiple arguments,
 * create a new type for the variables and use that as the first argument instead.
 *
 * For example:
 *
 * ```ts
 * function createUser(
 *   user: User,
 *   excludeUserField: ExcludeUserField,
 *   mfaResponse?: MfaChallengeResponse
 * ) { }
 * ```
 *
 * would become:
 *
 * ```ts
 * function createUser({ user, excludeUserField, mfaResponse }: CreateUserVariables) {}
 * ```
 *
 * You can also add the signal as the last argument if it is missing.
 *
 * @example
 *
 * This example shows how to create a query hook for fetching users. This
 * endpoint takes no variables, so there is no need to export the `createQueryKey`
 * method, as the query key will always be the same.
 *
 * ```tsx
 * const { queryKey: GetUsersQueryKey, useQuery: useGetUsers } = wrapQuery(
 *   ['users', 'get'],
 *   userService.fetchUsers
 * );
 * ```
 *
 * Usage:
 *
 * ```tsx
 * function UserList() {
 *   const queryClient = useQueryClient();
 *
 *   const { data, error, isPending } = useGetUsers();
 *
 *   const handleRemoveUser = useCallback((userId: string) => {
 *     queryClient.setQueryData(GetUsersQueryKey, previous => {
 *       // previous is automatically inferred as User[]
 *
 *       return previous.filter(user => user.id !== userId);
 *     });
 *   }, []);
 * }
 * ```
 *
 * This is instead of doing this:
 *
 * ```tsx
 * const queryKey = ['users', 'get'];
 *
 * function UserList() {
 *   const { data, error, isPending } = useQuery({
 *     queryKey,
 *     queryFn: () => userService.fetchUsers(), // TanStack Query passes the context first, so we need to ignore it by wrapping the function call.
 *   });
 *
 *   const handleRemoveUser = useCallback((userId: string) => {
 *     queryClient.setQueryData(queryKey, (previous: User[]) => {
 *       return previous.filter(user => user.id !== userId);
 *     });
 *   }, []);
 * }
 * ```
 *
 * @example
 *
 * This example shows how to create a query hook for fetching a user. This endpoint
 * takes the user ID as a variable, so we would need to export the `createQueryKey`
 * method as well.
 *
 * ```tsx
 * const {
 *   createQueryKey: createGetUserQueryKey,
 *   queryKey: GetUsersQueryKey,
 *   useQuery: useGetUsers
 * } = wrapQuery(
 *   ['user', 'get'],
 *   userService.fetchUser
 * );
 * ```
 *
 * `userService.fetchUser` would look like `(userId: string, signal: AbortSignal) => Promise<User>`.
 *
 * Usage:
 *
 * ```tsx
 * function UserDetails({ userId }: { userId: string }) {
 *   const queryClient = useQueryClient();

 *   const { data, error, isPending } = useGetUser(userId);
 *
 *   const handleNameChange = useCallback((name: string) => {
 *     const queryKey = createGetUserQueryKey(userId);
 *
 *     queryClient.setQueryData(queryKey, previous => {
 *       // previous is automatically inferred as User
 *
 *       return {
 *         ...previous,
 *         name,
 *       };
 *     });
 *   }, [userId]);
 * }
 * ```
 *
 * This is instead of doing this:
 *
 * ```tsx
 * function UserDetails({ userId }: { userId: string }) {
 *   // memoize the query key to avoid creating a new one and a new
 *   // `handleNameChange` function on every render.
 *   const queryKey = useMemo(() => ['user', 'get', userId], [userId]);
 *
 *   const { data, error, isPending } = useQuery({
 *     queryKey,
 *     queryFn: ({ signal }) => userService.fetchUser(userId, signal),
 *   }):
 *
 *   const handleNameChange = useCallback((name: string) => {
 *     queryClient.setQueryData(queryKey, (previous: User) => {
 *       return {
 *         ...previous,
 *         name,
 *       };
 *     });
 *   }, [queryKey]);
 * }
 * ```
 */
export function createQueryHook<
  TData = unknown,
  TVariables = void,
  TError = DefaultError,
  TPageParam = string,
>(
  queryKey: string[],
  queryFn: QueryFn<TData, TVariables>,
  addPageParamToVariables?: (
    pageParam: TPageParam,
    variables: TVariables
  ) => TVariables
): WrappedQuery<TData, TVariables, TError, TPageParam> {
  const wrapped: WrappedQuery<TData, TVariables, TError, TPageParam> = {
    queryKey: queryKey as DataTag<string[], TData, TError>,
    createQueryKey(variables?: TVariables) {
      const key = [...queryKey];

      if (variables) {
        key.push(JSON.stringify(variables));
      }

      return key as DataTag<string[], TData, TError>;
    },
    createQuery: function createQuery(variables?: TVariables) {
      return {
        queryKey: wrapped.createQueryKey(variables),
        queryFn: wrapped.queryFn(variables),
      };
    },
    queryFn:
      variables =>
      ({ signal }) =>
        callQueryFn(queryFn, variables, signal),
    useQuery: function wrappedQuery(
      variables?: TVariables,
      options?: Omit<UseQueryOptions<TData, TError>, 'queryKey' | 'queryFn'>
    ) {
      return reactQuery.useQuery({
        ...wrapped.createQuery(variables),
        ...options,
      });
    },
    useSuspenseQuery: function wrappedSuspenseQuery(
      variables?: TVariables,
      options?: Omit<UseQueryOptions<TData, TError>, 'queryKey' | 'queryFn'>
    ) {
      return reactQuery.useSuspenseQuery({
        ...wrapped.createQuery(variables),
        ...options,
      });
    },
    useInfiniteQuery: function wrappedInfiniteQuery(
      variables: TVariables,
      options: Omit<
        UseInfiniteQueryOptions<
          TData,
          TError,
          InfiniteData<TData, TPageParam>,
          DataTag<string[], TData, TError>,
          TPageParam
        >,
        'queryKey' | 'queryFn'
      >
    ) {
      return reactQuery.useInfiniteQuery<
        TData,
        TError,
        InfiniteData<TData, TPageParam>,
        DataTag<string[], TData, TError>,
        TPageParam
      >({
        ...options,
        queryKey: wrapped.createQueryKey(variables),
        queryFn: ({ pageParam, signal }) =>
          callQueryFn(
            queryFn,
            addPageParamToVariables
              ? addPageParamToVariables(pageParam as TPageParam, variables)
              : variables,
            signal
          ),
      });
    },
    useSuspenseInfiniteQuery: function wrappedSuspenseInfiniteQuery(
      variables: TVariables,
      options: Omit<
        UseSuspenseInfiniteQueryOptions<
          TData,
          TError,
          InfiniteData<TData, TPageParam>,
          DataTag<string[], TData, TError>,
          TPageParam
        >,
        'queryKey' | 'queryFn'
      >
    ) {
      return reactQuery.useSuspenseInfiniteQuery<
        TData,
        TError,
        InfiniteData<TData, TPageParam>,
        DataTag<string[], TData, TError>,
        TPageParam
      >({
        ...options,
        queryKey: wrapped.createQueryKey(variables),
        queryFn: ({ pageParam, signal }) =>
          callQueryFn(
            queryFn,
            addPageParamToVariables
              ? addPageParamToVariables(pageParam as TPageParam, variables)
              : variables,
            signal
          ),
      });
    },
  };

  return wrapped;
}

function isSignalOnlyQueryFn<TData = unknown, TVariables = void>(
  queryFn: QueryFn<TData, unknown>,
  variables: TVariables
): queryFn is SignalOnlyQueryFn<TData> {
  return typeof variables === 'undefined';
}

function callQueryFn<TData = unknown, TVariables = void>(
  queryFn: QueryFn<TData, TVariables>,
  variables: TVariables,
  signal: AbortSignal
) {
  if (isSignalOnlyQueryFn(queryFn, variables)) {
    return queryFn(signal);
  }

  return queryFn(variables, signal);
}
