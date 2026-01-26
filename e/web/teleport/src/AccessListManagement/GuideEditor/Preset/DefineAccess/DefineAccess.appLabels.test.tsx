import { act, within } from '@testing-library/react';
import { mockIntersectionObserver } from 'jsdom-testing-mocks';
import { setupServer } from 'msw/node';
import selectEvent from 'react-select-event';

import {
  render,
  screen,
  testQueryClient,
  userEvent,
} from 'design/utils/testing';

import ResourceService from 'teleport/services/resources';

import { fetchUnifiedResources, makeHandlers } from '../TestHelper/mocks';
import { ProviderWithQuery } from '../TestHelper/ProviderWithQuery';
import { DefineAccess } from './DefineAccess';

const server = setupServer();
const mio = mockIntersectionObserver();

beforeAll(() => {
  server.listen();
});

let spiedUnifiedResource;
beforeEach(() => {
  server.use(...makeHandlers([fetchUnifiedResources('get', null)]));
  spiedUnifiedResource = jest.spyOn(
    ResourceService.prototype,
    'fetchUnifiedResources'
  );
});

afterEach(async () => {
  server.resetHandlers();
  await testQueryClient.resetQueries();

  jest.clearAllMocks();
});

afterAll(() => {
  server.close();
});

// Does focus testing on applications, since other "label" based resources
// uses the same label input box.
test(`typing, clicking, and deleting labels`, async () => {
  const user = userEvent.setup();

  render(
    <ProviderWithQuery>
      <DefineAccess />
    </ProviderWithQuery>
  );

  await screen.findByText(/define application access/i);
  act(mio.enterAll); // trigger the isIntersecting of IntersectionObserver

  await screen.findByText(/no access defined/i);

  spiedUnifiedResource.mockClear();

  /**
   * Clicking a label, renders it in the input box.
   */
  let targetRow = screen.getByTestId('AppTestRow');
  await user.click(within(targetRow).getByTitle(/env: test/i));
  act(mio.enterAll);
  await screen.findByText(/AppTestRow2/i);

  expect(screen.queryByText(/no access defined/i)).not.toBeInTheDocument();
  expect(
    screen.queryByText(/type a label and press enter/i)
  ).not.toBeInTheDocument();

  const inputWrapper = screen.getByTestId('resource-label-input');
  expect(within(inputWrapper).getByText(/env: test/i)).toBeInTheDocument();

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      query:
        '((labels["env"] == "test")) && labels["teleport.dev/origin"] != "aws-identity-center"',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();

  /**
   * Clicking a label with same key but different
   * value updates input box with OR statement
   */
  targetRow = screen.getByTestId('AppTestRow2');
  await user.click(within(targetRow).getByText(/env: test2/i));
  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  expect(
    within(inputWrapper).getByText(/env: test OR test2/i)
  ).toBeInTheDocument();

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      query:
        '((labels["env"] == "test" || labels["env"] == "test2")) && labels["teleport.dev/origin"] != "aws-identity-center"',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();

  /**
   * Add a third OR statement by typing the label
   * instead of clicking labels.
   */
  await user.type(inputWrapper, 'env: test3');
  await selectEvent.select(inputWrapper, 'env: test3');

  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  expect(
    within(inputWrapper).getByText(/env: test OR test2 OR test3/i)
  ).toBeInTheDocument();

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      query:
        '((labels["env"] == "test" || labels["env"] == "test2" || labels["env"] == "test3")) && labels["teleport.dev/origin"] != "aws-identity-center"',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();

  /**
   * Clicking a different label updates input box with AND statement
   */
  targetRow = screen.getByTestId('AppTestRow');
  await user.click(within(targetRow).getByText(/test: apple/i));
  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  expect(
    within(inputWrapper).getByText(/env: test OR test2 OR test3/i)
  ).toBeInTheDocument();
  expect(within(inputWrapper).getByText(/test: apple/i)).toBeInTheDocument();
  expect(within(inputWrapper).getByText('AND')).toBeInTheDocument();

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      query:
        '((labels["env"] == "test" || labels["env"] == "test2" || labels["env"] == "test3") && (labels["test"] == "apple")) && labels["teleport.dev/origin"] != "aws-identity-center"',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();

  /**
   * Typing a different label updates input box with another
   * AND statement
   */
  await user.type(inputWrapper, 'test2: banana');
  await selectEvent.select(inputWrapper, 'test2: banana');

  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  expect(
    within(inputWrapper).getByText(/env: test OR test2 OR test3/i)
  ).toBeInTheDocument();
  expect(within(inputWrapper).getByText(/test: apple/i)).toBeInTheDocument();
  expect(within(inputWrapper).getByText(/test2: banana/i)).toBeInTheDocument();
  expect(within(inputWrapper).getAllByText('AND')).toHaveLength(2);

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      query:
        '((labels["env"] == "test" || labels["env"] == "test2" || labels["env"] == "test3") && (labels["test"] == "apple") && (labels["test2"] == "banana")) && labels["teleport.dev/origin"] != "aws-identity-center"',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();

  /**
   * Deleting a label removes it from the input box
   */
  await user.click(
    within(inputWrapper).getByRole('button', {
      name: 'Remove test2: banana',
    })
  );
  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  expect(
    within(inputWrapper).getByText(/env: test OR test2 OR test3/i)
  ).toBeInTheDocument();
  expect(within(inputWrapper).getByText(/test: apple/i)).toBeInTheDocument();
  expect(
    within(inputWrapper).queryByText(/test2: banana/i)
  ).not.toBeInTheDocument();
  expect(within(inputWrapper).getAllByText('AND')).toHaveLength(1);

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(1);
  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      query:
        '((labels["env"] == "test" || labels["env"] == "test2" || labels["env"] == "test3") && (labels["test"] == "apple")) && labels["teleport.dev/origin"] != "aws-identity-center"',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();

  /**
   * Typing a label that was already selected should result in error
   */
  await user.type(inputWrapper, 'env: test3');
  await selectEvent.select(inputWrapper, 'env: test3');

  screen.getByText(/label "env: test3", value "test3" is already selected/i);
  expect(
    within(inputWrapper).getByText(/env: test OR test2 OR test3/i)
  ).toBeInTheDocument();
  expect(within(inputWrapper).getByText(/test: apple/i)).toBeInTheDocument();
  expect(within(inputWrapper).getAllByText('AND')).toHaveLength(1);

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(0);

  /**
   * Typing a invalid formatted label result in error
   */
  const reactSelectInput = screen.getByRole('combobox');
  await user.type(reactSelectInput, 'invalid');
  await selectEvent.select(reactSelectInput, /invalid/i);

  screen.getByText(/label "invalid" is invalid/i);
  expect(
    within(inputWrapper).getByText(/env: test OR test2 OR test3/i)
  ).toBeInTheDocument();
  expect(within(inputWrapper).getByText(/test: apple/i)).toBeInTheDocument();
  expect(within(inputWrapper).getAllByText('AND')).toHaveLength(1);

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(0);

  /**
   * Typing a another invalid formatted label result in error
   */
  await user.type(reactSelectInput, 'invalid:invalid');
  await selectEvent.select(reactSelectInput, /invalid:invalid/i);

  screen.getByText(/label "invalid:invalid" is invalid/i);
  expect(
    within(inputWrapper).getByText(/env: test OR test2 OR test3/i)
  ).toBeInTheDocument();
  expect(within(inputWrapper).getByText(/test: apple/i)).toBeInTheDocument();
  expect(within(inputWrapper).getAllByText('AND')).toHaveLength(1);

  expect(spiedUnifiedResource).toHaveBeenCalledTimes(0);

  /**
   * Using a wildcard for a label value with an existing key, replaces
   * its value with wildcard eg: `env: test OR test2 OR test3` should be
   * replaced with just `env: *`
   */
  await user.type(reactSelectInput, 'env: *');
  let wildCardOpt = await screen.findByText(/use wildcard/i);
  expect(screen.getByText(/by any value/i)).toBeInTheDocument();
  await selectEvent.select(wildCardOpt, /use wildcard/i);

  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  expect(within(inputWrapper).getByText('env: *')).toBeInTheDocument();
  expect(within(inputWrapper).getByText(/test: apple/i)).toBeInTheDocument();
  expect(within(inputWrapper).getAllByText('AND')).toHaveLength(1);
  expect(within(inputWrapper).queryByText('OR')).not.toBeInTheDocument();
  expect(within(inputWrapper).queryByText('test2')).not.toBeInTheDocument();
  expect(within(inputWrapper).queryByText('test3')).not.toBeInTheDocument();

  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      query:
        '((labels["test"] == "apple") && exists(labels["env"])) && labels["teleport.dev/origin"] != "aws-identity-center"',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();

  /**
   * Using a wildcard for key AND value should wipe out previous labels
   */
  await user.type(reactSelectInput, '*');
  wildCardOpt = await screen.findByText(/use wildcard/i);
  expect(screen.getByText(/match by any labels/i)).toBeInTheDocument();
  await selectEvent.select(wildCardOpt, /use wildcard/i);

  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  expect(within(inputWrapper).getByText('*: *')).toBeInTheDocument();
  expect(
    within(inputWrapper).queryByText(/test: apple/i)
  ).not.toBeInTheDocument();
  expect(within(inputWrapper).queryByText('AND')).not.toBeInTheDocument();
  expect(within(inputWrapper).queryByText('OR')).not.toBeInTheDocument();

  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      // for unified resource api, blank filter is same as "wildcard"
      query: '',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();

  /**
   * Removing all labels should go back to expected empty state.
   */
  await user.click(
    within(inputWrapper).getByRole('button', {
      name: 'Remove *: *',
    })
  );
  act(mio.enterAll);
  await screen.findByText(/AppTestRow3/i);

  expect(screen.getByText(/no access defined/i)).toBeInTheDocument();
  expect(screen.getByText(/type a label and press enter/i)).toBeInTheDocument();
  expect(within(inputWrapper).queryByText('*: *')).not.toBeInTheDocument();

  expect(spiedUnifiedResource).toHaveBeenCalledWith(
    expect.anything(),
    {
      kinds: ['app'],
      limit: 48,
      // this is the default filter for "application tab"
      query: 'labels["teleport.dev/origin"] != "aws-identity-center"',
      startKey: '',
      sort: { dir: 'ASC', fieldName: 'name' },
      search: undefined,
    },
    expect.anything()
  );
  spiedUnifiedResource.mockClear();
});
