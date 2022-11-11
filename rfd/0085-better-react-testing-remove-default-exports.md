---
authors: Ryan Clark (ryan.clark@goteleport.com)
state: draft
---

# RFD 85 - Better React testing, remove default exports

## What

Improve the webapps codebase by changing the way we test our components and write our component stories.

## Why

The current convention is to use Storybook stories to test components in their different states with the use of snapshots.  This has a
couple of knock-on effects.

### Component Architecture

As it's hard to mock data in Storybook, we end up exporting a component that is suitable for rendering in Storybook,
such as:

```typescript jsx
export function DesktopSession(props: State) {
  const {
    directorySharingState,
    setDirectorySharingState,
    clipboardState,
    fetchAttempt,
    tdpConnection,
    disconnected,
    wsConnection,
    setTdpConnection,
  } = props;
  // etc

  return (
    <Flex flexDirection="column">
      // etc
    </Flex>
  );
}
```

This component takes all of its state and business logic from the props passed in.

For use in the actual application, we wrap the component above in a default export which populates the props the
component needs by using a hook.

```typescript jsx
export default function Container() {
  const state = useDesktopSession();
  return <DesktopSession {...state} />;
}
```


### Testing

As a result of the above, we end up testing the `DesktopSession` component instead of the actual component (`Container`) that is
rendered in the UI.

A story may look like this:

```typescript jsx
export const FetchError = () => (
  <DesktopSession
    {...props}
    fetchAttempt={{ status: 'failed', statusText: 'some fetch  error' }}
    tdpConnection={{ status: 'success' }}
    wsConnection={'open'}
    disconnected={false}
  />
);
```

Which is then tested like so:

```typescript jsx
test('fetch error', () => {
  const { getByTestId } = render(<FetchError />);
  expect(getByTestId('Modal')).toMatchSnapshot();
});
```

This results in all the logic in `useDesktopSession` going untested. We're only testing the visual part of the UI,
but skipping all of the business logic.

## Details

### Changing how we test the visual UI

Experience tells us that Stories in Storybook are used most effectively for documenting how to use reusable components, and visually testing the UI. While using snapshot tests of stories is a rough approximation of a visual UI test, they come a lot of baggage: https://medium.com/@sapegin/whats-wrong-with-snapshot-tests-37fbe20dfe8e.

It might be worthwhile looking into something like [Chromatic](https://www.chromatic.com/) for visual UI testing.

### Focusing on behavioral tests

We should be testing all the different state possibilities in Jest and React Testing Library, mocking any data or
network requests to cause the component to render into the state we want. This ensures that the internal logic
of our components (generally the most critical and difficult-to-get-right aspect of frontend code) gets tested,
as well as the final UI state.

Tests should check for elements that exist for the specific state being tested, i.e. an error message when the data has
failed to load. Elements should be clicked on or events triggered in order to change the state of the component.

### Changing how we export components

Default exports should be avoided except when absolutely necessary (for example, with
`React.Lazy`), as they degrade the developer experience
for no discernible benefit. For more fully fleshed out reasoning, refer to the articles below:
- https://blog.neufund.org/why-we-have-banned-default-exports-and-you-should-do-the-same-d51fdc2cf2ad
- https://rajeshnaroth.medium.com/avoid-es6-default-exports-a24142978a7a
- https://blog.piotrnalepa.pl/2020/06/26/default-exports-vs-named-exports/
- https://ilikekillnerds.com/2019/08/default-exports-bad/

Instead, named exports should be used.

```typescript jsx
export function SomeComponent() {
  return (
    <div>
      hello!
    </div>
  );
}

export const SomeValue = 42;
```

```typescript jsx
import { SomeComponent, SomeValue } from './SomeComponent';
```

### Code Coverage

Our current code coverage is:

```
=============================== Coverage summary ===============================
Statements   : 36.12% ( 3843/10639 )
Branches     : 39.44% ( 1484/3762 )
Functions    : 32.36% ( 1376/4251 )
Lines        : 36.31% ( 3759/10350 )
================================================================================
```

We should add code coverage to CI in order to track progress. There's a [GitHub Action](https://github.com/marketplace/actions/code-coverage-report)
that will comment on a PR with the coverage results, and a coverage UI of all files can be generated locally via `jest`.

### How to implement these changes

From this RFD, all new code to the webapps repo should follow this convention. An effort to go back and improve coverage
for untested parts of the codebase in between project work would help to fix existing tests and improve our codebase.

Having a robust test suite would ensure that future tests are written well, as there will be plenty of examples to use.
