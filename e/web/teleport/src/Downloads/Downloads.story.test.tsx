import React from 'react';
import { render } from 'design/utils/testing';

import {
  LoadedLinux,
  LoadedWindows,
  LoadedMacOS,
  FailedLicense,
  FailedReleases,
  LoadingReleases,
  GeneratingLicense,
  GeneratedLicense,
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

test('renders when generating license', async () => {
  const { container } = render(<GeneratingLicense />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders when license generated', async () => {
  const { container } = render(<GeneratedLicense />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders when releases failed', async () => {
  const { container } = render(<FailedReleases />);
  expect(container.firstChild).toMatchSnapshot();
});

test('renders when releases are loading', async () => {
  const { container } = render(<LoadingReleases />);
  expect(container.firstChild).toMatchSnapshot();
});
