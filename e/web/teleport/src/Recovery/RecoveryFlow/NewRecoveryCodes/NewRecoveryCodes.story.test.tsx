import { render } from 'design/utils/testing';

import { Loaded, Failed } from './NewRecoveryCodes.story';

// TODO(bl-nero): Snapshot tests removed. Tests to be replaced by Storybook
// snapshot tests. See https://github.com/gravitational/teleport/issues/19185.

test('render recovery codes', () => {
  render(<Loaded />);
});

test('render failed state for recovery codes', () => {
  render(<Failed />);
});
