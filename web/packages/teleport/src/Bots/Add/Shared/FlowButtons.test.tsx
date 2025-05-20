/**
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import { createMemoryHistory } from 'history';
import { MemoryRouter } from 'react-router';

import { render, screen, userEvent } from 'design/utils/testing';

import cfg from 'teleport/config';

import { FlowButtons, FlowButtonsProps } from './FlowButtons';

describe('flowButtons component', () => {
  let props: FlowButtonsProps;
  beforeEach(() => {
    props = {
      backButton: {
        disabled: false,
        hidden: false,
      },
      nextButton: {
        disabled: false,
        hidden: false,
      },
      nextStep: () => {},
      prevStep: () => {},
      isFirstStep: false,
      isLastStep: false,
    };
  });

  it('disables the buttons according to the props', () => {
    render(
      <MemoryRouter
        future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
      >
        <FlowButtons
          {...props}
          backButton={{ disabled: true }}
          nextButton={{ disabled: true }}
        />
      </MemoryRouter>
    );

    expect(screen.getByTestId('button-back')).toBeDisabled();
    expect(screen.getByTestId('button-next')).toBeDisabled();
  });

  it('hides the buttons according to the props', () => {
    render(
      <MemoryRouter
        future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
      >
        <FlowButtons
          {...props}
          backButton={{ hidden: true }}
          nextButton={{ hidden: true }}
        />
      </MemoryRouter>
    );

    expect(screen.queryByTestId('button-back')).not.toBeInTheDocument();
    expect(screen.queryByTestId('button-next')).not.toBeInTheDocument();
  });

  it('triggers the correct click handler', async () => {
    const prevMock = jest.fn();
    const nextMock = jest.fn();
    render(
      <MemoryRouter
        future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
      >
        <FlowButtons {...props} prevStep={prevMock} nextStep={nextMock} />
      </MemoryRouter>
    );

    await userEvent.click(screen.getByTestId('button-back'));
    expect(prevMock).toHaveBeenCalledTimes(1);

    await userEvent.click(screen.getByTestId('button-next'));
    expect(nextMock).toHaveBeenCalledTimes(1);
  });

  test('when useLocation.state is defined, the first step back button uses this state as pathname', async () => {
    const history = createMemoryHistory({
      initialEntries: [{ state: { previousPathname: 'web/random/route' } }],
    });
    history.push = jest.fn();

    render(
      <MemoryRouter
        future={{ v7_startTransition: true, v7_relativeSplatPath: true }}
        initialEntries={history.entries}
      >
        <FlowButtons {...props} isFirstStep={true} />
      </MemoryRouter>
    );

    const backLink = screen.getByTestId('button-back-first-step');
    expect(backLink).toHaveAttribute('href', '/web/random/route');
  });

  test('when useLocation.state is NOT defined, the first step back button defaults to bots pathname', async () => {
    const history = createMemoryHistory();

    render(
      <MemoryRouter
        future={{ v7_relativeSplatPath: true, v7_startTransition: true }}
        initialEntries={history.entries}
      >
        <FlowButtons {...props} isFirstStep={true} />
      </MemoryRouter>
    );

    const backLink = screen.getByTestId('button-back-first-step');
    expect(backLink).toHaveAttribute('href', cfg.getBotsNewRoute());
  });
});
