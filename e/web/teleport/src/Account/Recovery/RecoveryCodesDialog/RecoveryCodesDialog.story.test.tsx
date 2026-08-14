import { render } from 'design/utils/testing';

import { Failed, Loaded, LoadedFirstTime } from './RecoveryCodesDialog.story';

// TODO(bl-nero): Snapshost tests have been removed, replace them with Storybook smoke tests.
// See https://github.com/gravitational/teleport/issues/19185.

test('render successful recovery codes dialog', () => {
  render(<Loaded />);
});

test('render successful first timer recovery codes dialog', () => {
  render(<LoadedFirstTime />);
});

test('render failed state of recovery codes dialog', () => {
  render(<Failed />);
});
