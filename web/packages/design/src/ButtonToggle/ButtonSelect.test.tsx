import { render, screen } from 'design/utils/testing';

import { ButtonSelect } from './ButtonSelect';

describe('ButtonToggle', () => {
  const options = [
    { key: '1', label: 'Option 1' },
    { key: '2', label: 'Option 2' },
    { key: '3', label: 'Option 3' },
  ];
  const activeOption = '1';

  function renderButtonSelect() {
    const onChange = jest.fn();
    render(
      <ButtonSelect
        options={options}
        activeOption={activeOption}
        onChange={onChange}
      />
    );
    return { buttons: screen.getAllByRole('button'), onChange };
  }

  it('renders buttons with correct labels', () => {
    const { buttons } = renderButtonSelect();
    expect(buttons).toHaveLength(options.length);
    expect(buttons[0]).toHaveTextContent('Option 1');
    expect(buttons[1]).toHaveTextContent('Option 2');
    expect(buttons[2]).toHaveTextContent('Option 3');
  });

  it('applies data-active attribute correctly', () => {
    const { buttons } = renderButtonSelect();
    expect(buttons).toHaveLength(options.length);
    expect(buttons[0]).toHaveAttribute('data-active', 'true');
    expect(buttons[1]).toHaveAttribute('data-active', 'false');
    expect(buttons[2]).toHaveAttribute('data-active', 'false');
  });

  it('calls onChange with the correct key when a button is clicked', () => {
    const { buttons, onChange } = renderButtonSelect();
    buttons[1].click();
    expect(onChange).toHaveBeenCalledWith('2');
  });

  it('does not call onChange if the clicked button is already active', () => {
    const { buttons, onChange } = renderButtonSelect();
    buttons[0].click();
    expect(onChange).not.toHaveBeenCalled();
  });
});
