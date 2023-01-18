import React from 'react';
import { render } from 'design/utils/testing';

import {
  LoadedLinux,
  LoadedWindows,
  LoadedMacOS,
  FailedLicense,
  FailedReleases,
  LoadingLicense,
  LoadingReleases,
} from './Downloads.story';

test('renders loaded linux', async () => {
  const { container } = render(<LoadedLinux />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders loaded windows', async () => {
  const { container } = render(<LoadedWindows />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders loaded macOS', async () => {
  const { container } = render(<LoadedMacOS />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders when license failed', async () => {
  const { container } = render(<FailedLicense />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders when releases failed', async () => {
  const { container } = render(<FailedReleases />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders when license is loading', async () => {
  const { container } = render(<LoadingLicense />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders when releases are loading', async () => {
  const { container } = render(<LoadingReleases />);
  expect(container.firstChild).toMatchSnapshot();
});
