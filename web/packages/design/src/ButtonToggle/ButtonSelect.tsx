import styled from 'styled-components';

import { ButtonPrimary } from '../Button/Button';

interface ButtonSelectProps {
  // The options to display in the button select. Each option should have a unique `key` and a `label` to display on the button.
  options: { key: string; label: string }[];
  // The key of the active option
  activeOption: string;
  // Callback function that is called when the active button changes. It receives the key of the newly selected button.
  onChange: (selectedKey: string) => void;
}

/**
 * ButtonSelect is a segmented button that allows users to select one of the provided options.
 *
 * Each option must have a unique `key` and a `label` that will be displayed on the button.
 *
 * @example
 * const options = [
 *   { key: '1', label: 'Option 1' },
 *   { key: '2', label: 'Option 2' },
 * ];
 * const [activeOption, setActiveOption] = useState('1');
 * return (
 *   <ButtonSelect
 *     options={options}
 *     activeOption={activeOption}
 *     onChange={setActiveOption}
 *   />
 * );
 */

export const ButtonSelect = ({
  options,
  activeOption,
  onChange,
}: ButtonSelectProps) => {
  const activeIndex = options.findIndex(option => option.key === activeOption);

  const updateValue = (newOption: string) => {
    if (activeOption !== newOption) {
      onChange(newOption);
    }
  };

  function handleKeyDown(e: React.KeyboardEvent) {
    if (activeIndex === -1) return;
    if (e.key === 'ArrowRight' && activeIndex < options.length - 1) {
      onChange(options[activeIndex + 1].key);
    }
    if (e.key === 'ArrowLeft' && activeIndex > 0) {
      onChange(options[activeIndex - 1].key);
    }
  }

  return (
    <ButtonSelectWrapper
      tabIndex={0}
      onKeyDown={handleKeyDown}
      data-testid="button-select"
    >
      {options.map((option, index) => {
        const isActive = activeOption === option.key;
        return (
          <ButtonSelectButton
            key={option.key}
            onClick={() => updateValue(option.key)}
            data-testid={`button-toggle-option-${index}`}
            tabIndex={-1}
            isActive={isActive}
            data-active={isActive}
            intent={isActive ? 'primary' : 'neutral'}
          >
            {option.label}
          </ButtonSelectButton>
        );
      })}
    </ButtonSelectWrapper>
  );
};

const ButtonSelectButton = styled(ButtonPrimary)<{ isActive: boolean }>`
  // middle button(s) style
  border-radius: 0px;

  // left button style
  &:first-child {
    border-top-left-radius: 4px;
    border-bottom-left-radius: 4px;
  }

  // right button style
  &:last-child {
    border-top-right-radius: 4px;
    border-bottom-right-radius: 4px;
  }
`;

const ButtonSelectWrapper = styled.div`
  display: flex;
  width: fit-content;
  border-radius: 4px;

  &:focus-visible {
    outline: 2px solid
      ${({ theme }) => theme.colors.interactive.solid.primary.default};
    outline-offset: 2px;
  }
`;
