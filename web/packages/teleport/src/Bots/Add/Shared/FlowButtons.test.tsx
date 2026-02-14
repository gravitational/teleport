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

import { MemoryRouter } from 'react-router';

import { render, screen, userEvent } from 'design/utils/testing';

import cfg from 'teleport/config';

import { FlowButtons, FlowButtonsProps } from './FlowButtons';

const mockedNavigate = jest.fn();
const mockedUseLocation = jest.fn();

jest.mock('react-router', () => ({
  ...jest.requireActual('react-router'),
  useNavigate: () => mockedNavigate,
  useLocation: () => mockedUseLocation(),
}));

describe('flowButtons component', () => {
  let props: FlowButtonsProps;
  beforeEach(() => {
    mockedNavigate.mockReset();
    mockedUseLocation.mockReset();
    mockedUseLocation.mockReturnValue({ key: 'test' });

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
      <MemoryRouter>
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
      <MemoryRouter>
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
      <MemoryRouter>
        <FlowButtons {...props} prevStep={prevMock} nextStep={nextMock} />
      </MemoryRouter>
    );

    await userEvent.click(screen.getByTestId('button-back'));
    expect(prevMock).toHaveBeenCalledTimes(1);

    await userEvent.click(screen.getByTestId('button-next'));
    expect(nextMock).toHaveBeenCalledTimes(1);
  });

  test('when there is a previous history location, then back button uses `history.goBack()`', async () => {
    render(
      <MemoryRouter
        initialEntries={[
          { pathname: '/', state: { previousPathname: 'web/random/route' } },
        ]}
      >
        <FlowButtons {...props} isFirstStep={true} />
      </MemoryRouter>
    );

    await userEvent.click(screen.getByTestId('button-back-first-step'));
    expect(mockedNavigate).toHaveBeenCalledWith(-1);
  });

  test('when there is no previous history location, then back button pushes to integrations list', async () => {
    mockedUseLocation.mockReturnValue({ key: 'default' });

    render(
      <MemoryRouter>
        <FlowButtons {...props} isFirstStep={true} />
      </MemoryRouter>
    );

    await userEvent.click(screen.getByTestId('button-back-first-step'));
    expect(mockedNavigate).toHaveBeenCalledWith(
      cfg.getIntegrationsEnrollRoute()
    );
  });
});
