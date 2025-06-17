---
authors: Ryan Clark (ryan.clark@goteleport.com)
state: draft
---

# RFD 0195 - Better async state management with TanStack Query

## Required Approvers

* Engineering: @ravicious || @kimlisa || @avatus

## What

Simplify data fetching and state management in the frontend by introducing TanStack Query.

## Why

TanStack Query is a very popular open-source library for managing async state in React applications, with over 44k stars
on GitHub.
It is a powerful tool that provides easy to use hooks for fetching and caching data from the backend.

## Current Setup

As it stands, data fetching in Teleport is done using one of two different ways:

### useAttempt

This is a custom hook that is used to manage any state in Teleport. The attempt can have different statuses (none,
loading, error, success).

This results in a lot of boilerplate code in managing the data fetching & state storage, and error handling.

#### Example query

```tsx
function fetchAccessList() {
  scopedAttempt.setAttempt({ status: 'processing' });

  accessManagementService
    .fetchAccessList(accessListId)
    .then(fetchedAccessList => {
      const [modifiedAccessList, newPerms] = modifyAccessList(
        fetchedAccessList,
        accessLists,
        ctx
      );
      setAccessList(modifiedAccessList);
      setPerms(newPerms);
      scopedAttempt.setAttempt({ status: 'success' });
    })
    .catch((e: Error) =>
      scopedAttempt.setAttempt({ status: 'failed', statusText: e.message })
    );
}
```

This then has to be called with an effect that watches the access list ID:

```tsx
// The accessListId can change if a user clicks on a different
// access list in the notification dropdown.
useEffect(() => {
  if (!accessList || accessList.id !== accessListId) {
    fetchAccessList();
  }
}, [accessListId]);
```

#### Example mutation

```tsx
const { attempt, setAttempt } = useAttempt();
const isDisabled = attempt.status === 'processing';

function onOk() {
  setAttempt({ status: 'processing' });
  accessManagementService
    .deleteAccessList(accessListId)
    .then(() =>
      history.push(cfg.getAccessListManagementRoute(), {
        deletedAccessListId: accessListId,
      })
    )
    .catch((e: Error) =>
      setAttempt({ status: 'failed', statusText: e.message })
    );
}
```

### useAsync

This is a newer custom hook that is used to manage async state in Teleport. It requires less boilerplate than
`useAttempt`, as it handles running async operations and handling errors, but still requires a manual trigger in an
effect,
which causes data to not fetch until at least the component has rendered once.

Whilst this works fine, it lacks an extended API that TanStack Query can provide.

#### Example query

```tsx
const [fetchQrCodeAttempt, fetchQrCode] = useAsync((privilegeToken: string) =>
  auth.createMfaRegistrationChallenge(privilegeToken, 'totp')
);

useEffect(() => {
  fetchQrCode(privilegeToken);
}, []);
```

#### Example mutation

```tsx
const [deleteRequestAttempt, runDeleteRequest] = useAsync(async () => {
  await ctx.workflowService.deleteAccessRequest(props.requestId);
  historyService.replace(cfg.getAccessRequestRoute());
});
```

## TanStack Query

TanStack Query offers many powerful abilities, including:

- Automatic retries and manual re-fetching methods
  - Automatic retries can be configured to retry failed requests a certain number of times, with exponential backoff, something not easily achievable
    with `useAttempt` or `useAsync`
- Caching and background re-fetching
  - There are no caching abilities on the frontend as it stands, so avoiding unnecessary network requests is a big win
- Infinite queries
  - This can allow us to create infinite scrolling tables, which is a better UX for larger customers
- Paginated queries
  - At the moment we have a large wrapper utility to add paginated query support for our tables. TanStack Query provides a built-in
    way to handle paginated queries, which is much easier to use and understand
- Parallel queries
  - TanStack Query can allow us to run multiple queries in parallel, which is useful for loading multiple pieces of data at once
- Suspense support
  - As it stands we're not using React's Suspense API, but TanStack Query has built-in support for it. This can allow us to use Suspense for data fetching, which can simplify our code and improve the user experience

### Suspense support

Suspense is a great way to fetch data, moving the loading and error states into the component tree rather than
every component that fetches data dealing with the loading and error states.

For example, instead of

```tsx
// pseudo code

const attempt = useAttempt();

if (attempt.status === 'processing') {
  return <Loading />;
}

if (attempt.status === 'failed') {
  return <Error />;
}

// continue rendering the component
```

The component can be wrapped in a `Suspense` component, which will show a loading state until the data is ready:

```tsx
<ErrorBoundary FallbackComponent={BotInformationError}>
  <Suspense fallback={<BotInformationSkeleton />}>
    <BotInformationInner {...props} />
  </Suspense>
</ErrorBoundary>
```

This keeps the logic of `BotInformationInner` tightly coupled to the data it needs, rather than having to deal with
loading and error states - they're handled by the component tree.

There are other benefits to using Suspense - it gives more control over the loading state when there are multiple
components that need to fetch data, as well as allowing for more granular error handling. [Read more here](https://react.dev/reference/react/Suspense).

### Example query

```tsx
const { data, error, isPending } = useQuery({
  queryKey: ['accessList', accessListId],
  queryFn: () => accessManagementService.fetchAccessList(accessListId),
});
```

Which could be wrapped as a hook for easy reuse:

```tsx
export function useGetAccessList(id: string) {
  return useQuery({
    queryKey: ['accessList', id],
    queryFn: () => accessManagementService.fetchAccessList(id),
  });
}
```

### Example mutation

```tsx
const { mutateAsync: deleteAccessList, isPending } = useMutation({
  mutationFn: (accessListId: string) =>
    accessManagementService.deleteAccessList(accessListId),
});

async function onOk() {
  await deleteAccessList(accessListId);

  history.push(cfg.getAccessListManagementRoute(), {
    deletedAccessListId: accessListId,
  });
}
```

### Example infinite query

```tsx
const { fetchNextPage, hasNextPage, data, isPending } = useInfiniteQuery({
  getNextPageParam: lastPage => lastPage.next_cursor,
  initialPageParam: '',
  queryFn,
  queryKey,
});
```

This allows the next page to be fetched through `fetchNextPage` without having to worry about the pagination cursor,
as well as an indication as to whether there is more data to fetch.

### Example paginated query

TanStack Query allows us to easily create paginated queries (and even pre-fetch the next page of results for faster
rendering),
as well as keeping the previous page's data whilst the next page is being fetched. This is not easily
achievable with the current `useAttempt` or `useAsync` methods.

```tsx
const [page, setPage] = useState(0);

const { data, error, isFetching, isPlaceholderData } = useQuery({
  queryKey: ['users', page],
  queryFn: () => fetchUsers(page),
  placeholderData: keepPreviousData,
})

useEffect(() => {
  if (!isPlaceholderData && data?.next_cursor) {
    queryClient.prefetchQuery({
      queryKey: ['users', page + 1],
      queryFn: () => fetchUsers(page + 1),
    })
  }
}, [data, isPlaceholderData, page, queryClient])
```

## Example Implementation

https://github.com/gravitational/teleport/pull/47282 shows an example of how we can introduce TanStack Query into the
codebase.

This can be incremental, with new pages using TanStack Query and old pages being converted over time.

Moving away from passing the result from `useAttempt` (or the "attempt" returned from `useAsync`) results in better
tested
code (the network requests will have to be mocked, which reflects the real world usage), as well as better stories
reflecting the actual state of the page,
rather than the state of the page with predefined props which may not reflect reality.