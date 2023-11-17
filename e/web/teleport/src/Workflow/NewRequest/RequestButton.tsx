import React from 'react';
import { ButtonPrimary, ButtonBorder } from 'design';

export const RequestButton = ({
  isAgentAdded,
  toggleAgent,
  disabled = false,
}: {
  isAgentAdded: boolean;
  toggleAgent: () => void;
  disabled?: boolean;
}) => {
  if (isAgentAdded) {
    return (
      <ButtonPrimary
        disabled={disabled}
        width="134px"
        size="small"
        onClick={toggleAgent}
      >
        Remove
      </ButtonPrimary>
    );
  }
  return (
    <ButtonBorder
      disabled={disabled}
      width="134px"
      size="small"
      onClick={toggleAgent}
    >
      + Add to request
    </ButtonBorder>
  );
};
