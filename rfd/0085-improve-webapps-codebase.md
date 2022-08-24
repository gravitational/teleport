---
authors: Ryan Clark (ryan.clark@goteleport.com)
state: draft
---

# RFD 85 - Improving the Webapps codebase

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

This causes issues when trying to navigate around the codebase and find instances of the actual React component that is
being rendered. Default exports are not recommended to be used unless absolutely necessary (for example, with 
`React.Lazy`).
- https://blog.neufund.org/why-we-have-banned-default-exports-and-you-should-do-the-same-d51fdc2cf2ad
- https://rajeshnaroth.medium.com/avoid-es6-default-exports-a24142978a7a
- https://blog.piotrnalepa.pl/2020/06/26/default-exports-vs-named-exports/
- https://ilikekillnerds.com/2019/08/default-exports-bad/

### Testing

As a result of the above, we end up testing the `DesktopSession` component instead of the actual component that is
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

This results in all the logic in `useDesktopSession` to be untested. We're only testing the UI part of the application,
instead of any business logic.

Storybook isn't designed for the way we are using it. As a result of putting our large components into stories, we have
ended up with only testing our UI and nothing else.

## Details

### Changing how we write stories

Stories in Storybook are designed for creating isolated, individual components. We should restrict usage of Storybook to
our shared component library.

### Changing how we write tests

We should be testing all the different state possibilities in Jest and React Testing Library, mocking any data or 
network requests to cause the component to render into the state we want.

We should avoid using snapshots when testing, if possible. Snapshots break constantly when styles are updated, and
don't provide any actual security around behaviour or appearance. Further reading - 
https://medium.com/@sapegin/whats-wrong-with-snapshot-tests-37fbe20dfe8e.

Tests should check for elements that exist for the specific state being tested, i.e. an error message when the data has 
failed to load. Elements should be clicked on or events triggered in order to change the state of the component.

### Changing how we export components

Default exports should be avoided at all costs.

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
for inspecting coverage issues. It can also annotate pull requests with coverage issues.

We should then implement Codecov's status check on pull requests. This check should fail if a certain % of the code 
touched by the pull request has not got coverage. This means that developers won't be punished for the lack of coverage
in the existing codebase, as it'll only check the coverage percentage of the changes in the pull request.

### How to implement these changes

From this RFD, all new code to the webapps repo should follow this convention. It would take a while to go back through
the existing codebase and make these changes, as it would involve rewriting the tests that already exist. An effort to
improve code coverage would be beneficial, but not a priority.
