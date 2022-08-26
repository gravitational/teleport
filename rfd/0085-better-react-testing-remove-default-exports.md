---
authors: Ryan Clark (ryan.clark@goteleport.com)
state: draft
---

# RFD 85 - Better React testing, remove default exports

## What

Improve the webapps codebase by changing the way we test our components and write our component stories.

## Why

The current convention is to use Storybook to render our large, full components in their different states.  This has a
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

This results in all the logic in `useDesktopSession` going untested. We're only testing the UI part of the application,
but skipping all of the business logic.

Storybook isn't best suited for the way we are using it. As a result of testing our large components primarily via stories, we have
ended up only testing our UI and nothing else.

## Details

### Changing how we write stories

Experience tells us that Stories in Storybook are used most effectively for documenting how to use reusable components. We should therefore
restrict usage of Storybook mostly to our shared component library, and test our fullscreen components another way.

It's still okay to create our larger components in Storybook (if the developer wants to) but we should not be using these stories in our Jest tests.

It might be worthwhile looking into something like [Chromatic](https://www.chromatic.com/) for visual UI testing.

### Changing how we write tests

We should be testing all the different state possibilities in Jest and React Testing Library, mocking any data or
network requests to cause the component to render into the state we want. This ensures that the internal logic
of our components (generally the most critical and difficult-to-get-right aspect of frontend code) gets tested,
as well as the final UI state.

Tests should check for elements that exist for the specific state being tested, i.e. an error message when the data has
failed to load. Elements should be clicked on or events triggered in order to change the state of the component.

By writing tests as described above, snapshot tests typically become redundant and therefore unnecessary. Besides the problem of skipping the business logic, snapshots consistently break for non-bugs, such as when styles are updated, and
generally don't provide much if any security around behaviour or appearance. Further reading -
https://medium.com/@sapegin/whats-wrong-with-snapshot-tests-37fbe20dfe8e.

### Changing how we export components

Default exports should be avoided except when absolutely necessary (for example, with
`React.Lazy`), as they degrade the developer experience
for no discernible benefit. For more fully fleshed out reasoning, refer to the articles below:
- https://blog.neufund.org/why-we-have-banned-default-exports-and-you-should-do-the-same-d51fdc2cf2ad
- https://rajeshnaroth.medium.com/avoid-es6-default-exports-a24142978a7a
- https://blog.piotrnalepa.pl/2020/06/26/default-exports-vs-named-exports/
- https://ilikekillnerds.com/2019/08/default-exports-bad/

When needing to export something as default, we should export it like so:

```typescript jsx
function SomeComponent() {
  return (
    <div>
      hello!
    </div>
  );
}

export { SomeComponent as default };
```

This provides a clearer, more explicit export.

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

We should add code coverage to CI in order to track progress. One tool we can use for this is
[Codecov](https://about.codecov.io/), which provide a free plan for open source tools. This would provide a useful UI
for inspecting coverage issues, and tracking our progress.

### How to implement these changes

From this RFD, all new code to the webapps repo should follow this convention. An effort to go back and improve coverage
for untested parts of the codebase in between project work would help to fix existing tests and improve our codebase.

Having a robust test suite would ensure that future tests are written well, as there will be plenty of examples to use.
