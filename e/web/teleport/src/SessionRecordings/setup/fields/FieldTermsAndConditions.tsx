import { useCallback, useId, type KeyboardEvent } from 'react';
import { useController } from 'react-hook-form';
import styled from 'styled-components';

import Box from 'design/Box';
import Flex from 'design/Flex';
import { Check } from 'design/Icon';
import ExternalLink from 'design/Link';
import Text from 'design/Text';
import { HelperTextLine } from 'shared/components/FieldInput/FieldInput';

interface FieldTermsAndConditionsProps {
  disabled?: boolean;
  name: string;
}

export function FieldTermsAndConditions({
  disabled,
  name,
}: FieldTermsAndConditionsProps) {
  const { field, fieldState, formState } = useController({
    name,
    defaultValue: false,
  });

  const helperTextId = useId();
  const isDisabled = formState.isSubmitting || formState.disabled || disabled;
  const checked = !!field.value;

  const handleToggle = useCallback(() => {
    field.onChange(!checked);
  }, [checked, field]);

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (event.key === 'Enter' || event.key === ' ') {
        event.preventDefault();
        handleToggle();
      }
    },
    [handleToggle]
  );

  return (
    <Box
      opacity={isDisabled ? 0.5 : 1}
      style={{ pointerEvents: isDisabled ? 'none' : 'auto' }}
    >
      <TermsContainer p={3} borderRadius={3}>
        <Text typography="body3" color="text.slightlyMuted">
          Session Recording Summaries sends sessions to an LLM. To use
          AI-powered features, please review and accept our{' '}
          <ExternalLink
            href="https://goteleport.com/legal/ai-features-terms/"
            target="_blank"
          >
            AI Features Terms
          </ExternalLink>
          .
        </Text>

        <BulletList>
          <li>
            <Text typography="body3" color="text.slightlyMuted">
              You&#39;re responsible for evaluating Session Recording Summary
              outputs for accuracy.
            </Text>
          </li>
          <li>
            <Text typography="body3" color="text.slightlyMuted">
              LLMs can make mistakes. Session Recording Summaries are a tool to
              support security response or auditing decisions. Verify accuracy
              before relying on results.
            </Text>
          </li>
        </BulletList>
      </TermsContainer>

      <Flex
        alignItems="center"
        gap={2}
        mt={3}
        role="checkbox"
        aria-checked={checked}
        aria-disabled={isDisabled}
        tabIndex={isDisabled ? -1 : 0}
        onClick={handleToggle}
        onKeyDown={handleKeyDown}
        css={`
          cursor: pointer;
          user-select: none;
        `}
      >
        <CheckboxSquare selected={checked}>
          {checked && <Check size="small" />}
        </CheckboxSquare>
        <Text typography="body2">
          I have read and agree to the{' '}
          <ExternalLink
            target="_blank"
            href="https://goteleport.com/legal/ai-features-terms/"
            onClick={e => e.stopPropagation()}
          >
            AI Features Terms
          </ExternalLink>
        </Text>
      </Flex>

      <HelperTextLine
        hasError={fieldState.invalid}
        helperTextId={helperTextId}
        helperText={undefined}
        errorMessage={fieldState.error?.message}
      />
    </Box>
  );
}

const TermsContainer = styled(Box)`
  background: ${p => p.theme.colors.interactive.tonal.neutral[0]};
  border: 1px solid ${p => p.theme.colors.interactive.tonal.neutral[1]};
`;

const BulletList = styled.ul`
  margin: ${p => p.theme.space[2]}px 0 0 0;
  padding-left: ${p => p.theme.space[3]}px;
  list-style: disc;
  display: flex;
  flex-direction: column;
  gap: ${p => p.theme.space[1]}px;
`;

const CheckboxSquare = styled.div<{ selected: boolean }>`
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 14px;
  height: 14px;
  border: 1px solid
    ${p =>
      p.selected
        ? p.theme.colors.interactive.solid.primary.default
        : p.theme.colors.interactive.tonal.neutral[2]};
  border-radius: 3px;
  background: ${p =>
    p.selected
      ? p.theme.colors.interactive.solid.primary.default
      : 'transparent'};
  color: ${p => p.theme.colors.text.primaryInverse};
`;
