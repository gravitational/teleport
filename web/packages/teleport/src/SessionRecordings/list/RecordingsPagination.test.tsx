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

import { render, screen, userEvent } from 'design/utils/testing';

import {
  RecordingsPagination,
  type RecordingsPaginationProps,
} from './RecordingsPagination';

const defaultProps: RecordingsPaginationProps = {
  count: 100,
  from: 0,
  to: 9,
  page: 0,
  pageSize: 10,
  onPageChange: jest.fn(),
};

function setupTest(props: Partial<RecordingsPaginationProps> = {}) {
  return render(<RecordingsPagination {...defaultProps} {...props} />);
}

afterEach(() => {
  jest.clearAllMocks();
});

function getPaginationIndicator() {
  return screen.getByTestId('recordings-pagination-indicator');
}

test('renders pagination with basic information', async () => {
  setupTest();

  expect(getPaginationIndicator()).toBeInTheDocument();
  expect(getPaginationIndicator()).toHaveTextContent('1 - 10 of 100');
  expect(screen.getByTitle('Previous page')).toBeInTheDocument();
  expect(screen.getByTitle('Next page')).toBeInTheDocument();
});

test('disables previous button on first page', async () => {
  setupTest({
    page: 0,
    from: 0,
    to: 9,
  });

  const prevButton = screen.getByTitle('Previous page');

  expect(prevButton).toBeDisabled();
});

test('disables next button on last page', async () => {
  setupTest({
    page: 9,
    from: 90,
    to: 99,
    count: 100,
  });

  const nextButton = screen.getByTitle('Next page');

  expect(nextButton).toBeDisabled();
});

test('enables both buttons on middle page', async () => {
  setupTest({
    page: 5,
    from: 50,
    to: 59,
  });

  const prevButton = screen.getByTitle('Previous page');
  const nextButton = screen.getByTitle('Next page');

  expect(prevButton).toBeEnabled();
  expect(nextButton).toBeEnabled();
});

test('calls onPageChange with previous page when clicking previous', async () => {
  const onPageChange = jest.fn();

  setupTest({
    page: 5,
    from: 50,
    to: 59,
    onPageChange,
  });

  const prevButton = screen.getByTitle('Previous page');
  await userEvent.click(prevButton);

  expect(onPageChange).toHaveBeenCalledWith(4);
});

test('calls onPageChange with next page when clicking next', async () => {
  const onPageChange = jest.fn();

  setupTest({
    page: 5,
    from: 50,
    to: 59,
    onPageChange,
  });

  const nextButton = screen.getByTitle('Next page');
  await userEvent.click(nextButton);

  expect(onPageChange).toHaveBeenCalledWith(6);
});

test('does not call onPageChange when clicking disabled previous button', async () => {
  const onPageChange = jest.fn();

  setupTest({
    page: 0,
    from: 0,
    to: 9,
  });

  const prevButton = screen.getByTitle('Previous page');
  await userEvent.click(prevButton);

  expect(onPageChange).not.toHaveBeenCalled();
});

test('does not call onPageChange when clicking disabled next button', async () => {
  const onPageChange = jest.fn();

  setupTest({
    page: 9,
    from: 90,
    to: 99,
    count: 100,
  });

  const nextButton = screen.getByTitle('Next page');
  await userEvent.click(nextButton);

  expect(onPageChange).not.toHaveBeenCalled();
});

test('shows fetch more button when fetchMoreAvailable is true', async () => {
  setupTest({
    fetchMoreAvailable: true,
    onFetchMore: jest.fn(),
  });

  expect(screen.getByText('Fetch More')).toBeInTheDocument();
});

test('does not show fetch more button when fetchMoreAvailable is false', async () => {
  setupTest({
    fetchMoreAvailable: false,
    onFetchMore: jest.fn(),
  });

  expect(screen.queryByText('Fetch More')).not.toBeInTheDocument();
});

test('does not show fetch more button when onFetchMore is not provided', async () => {
  setupTest({
    fetchMoreAvailable: true,
  });

  expect(screen.queryByText('Fetch More')).not.toBeInTheDocument();
});

test('calls onFetchMore when clicking fetch more button', async () => {
  const onFetchMore = jest.fn();

  setupTest({
    fetchMoreAvailable: true,
    onFetchMore,
  });

  const fetchMoreButton = screen.getByText('Fetch More');
  await userEvent.click(fetchMoreButton);

  expect(onFetchMore).toHaveBeenCalled();
});

test('disables fetch more button when fetchMoreDisabled is true', async () => {
  setupTest({
    fetchMoreAvailable: true,
    fetchMoreDisabled: true,
    onFetchMore: jest.fn(),
  });

  const fetchMoreButton = screen.getByText('Fetch More');

  expect(fetchMoreButton).toBeDisabled();
});

test('shows error message when fetchMoreError is true', async () => {
  setupTest({
    fetchMoreError: true,
  });

  expect(screen.getByText('An error occurred')).toBeInTheDocument();
});

test('shows retry button instead of fetch more when error occurs', async () => {
  const onFetchMore = jest.fn();

  setupTest({
    fetchMoreAvailable: true,
    fetchMoreError: true,
    onFetchMore,
  });

  expect(screen.getByText('Retry')).toBeInTheDocument();
  expect(screen.queryByText('Fetch More')).not.toBeInTheDocument();
});

test('calls onFetchMore when clicking retry button', async () => {
  const onFetchMore = jest.fn();

  setupTest({
    fetchMoreAvailable: true,
    fetchMoreError: true,
    onFetchMore,
  });

  const retryButton = screen.getByText('Retry');
  await userEvent.click(retryButton);

  expect(onFetchMore).toHaveBeenCalled();
});

test('displays correct page indicator for different pages', async () => {
  const { rerender } = setupTest({
    page: 0,
    from: 0,
    to: 9,
    count: 100,
  });

  expect(getPaginationIndicator()).toHaveTextContent('1 - 10 of 100');

  rerender(
    <RecordingsPagination
      {...defaultProps}
      onPageChange={jest.fn()}
      page={5}
      from={50}
      to={59}
      count={100}
    />
  );

  expect(getPaginationIndicator()).toHaveTextContent('51 - 60 of 100');

  rerender(
    <RecordingsPagination
      {...defaultProps}
      onPageChange={jest.fn()}
      page={9}
      from={90}
      to={99}
      count={100}
    />
  );

  expect(getPaginationIndicator()).toHaveTextContent('91 - 100 of 100');
});

test('handles partial last page correctly', async () => {
  setupTest({
    page: 10,
    from: 100,
    to: 104,
    count: 105,
    pageSize: 10,
  });

  expect(getPaginationIndicator()).toHaveTextContent('101 - 105 of 105');
  expect(screen.getByTitle('Next page')).toBeDisabled();
});

test('handles single page correctly', async () => {
  setupTest({
    page: 0,
    from: 0,
    to: 4,
    count: 5,
    pageSize: 10,
  });

  expect(getPaginationIndicator()).toHaveTextContent('1 - 5 of 5');
  expect(screen.getByTitle('Previous page')).toBeDisabled();
  expect(screen.getByTitle('Next page')).toBeDisabled();
});

test('does not show page indicator text when there are no results', async () => {
  setupTest({
    page: 0,
    from: 0,
    to: 0,
    count: 0,
    pageSize: 10,
  });

  expect(screen.queryByText('Showing')).not.toBeInTheDocument();
  expect(screen.getByTitle('Previous page')).toBeDisabled();
  expect(screen.getByTitle('Next page')).toBeDisabled();
});
