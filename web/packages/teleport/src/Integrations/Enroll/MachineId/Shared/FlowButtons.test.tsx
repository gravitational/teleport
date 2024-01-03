import { render, screen, userEvent } from 'design/utils/testing';
import React from 'react';

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
      isFirst: false,
      isLast: false,
    };
  });

  it('disables the buttons according to the props', () => {
    render(
      <FlowButtons
        {...props}
        backButton={{ disabled: true }}
        nextButton={{ disabled: true }}
      />
    );

    expect(screen.getByTestId('button-back')).toBeDisabled();
    expect(screen.getByTestId('button-next')).toBeDisabled();
  });

  it('hides the buttons according to the props', () => {
    render(
      <FlowButtons
        {...props}
        backButton={{ hidden: true }}
        nextButton={{ hidden: true }}
      />
    );

    expect(screen.queryByTestId('button-back')).not.toBeInTheDocument();
    expect(screen.queryByTestId('button-next')).not.toBeInTheDocument();
  });

  it('triggers the correct click handler', async () => {
    const prevMock = jest.fn();
    const nextMock = jest.fn();
    render(<FlowButtons {...props} prevStep={prevMock} nextStep={nextMock} />);

    await userEvent.click(screen.getByTestId('button-back'));
    expect(prevMock).toHaveBeenCalledTimes(1);

    await userEvent.click(screen.getByTestId('button-next'));
    expect(nextMock).toHaveBeenCalledTimes(1);
  });
});
