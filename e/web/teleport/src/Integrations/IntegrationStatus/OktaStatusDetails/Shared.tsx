import { formatDistanceStrict } from 'date-fns';
import { ComponentType, useRef, useState } from 'react';
import styled from 'styled-components';

import { Box, ButtonIcon, Flex, Label, Menu, MenuItem, Text } from 'design';
import { CircleCheck, CircleCross, MoreVert } from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { HoverTooltip, IconTooltip } from 'design/Tooltip';

import { PluginOktaSyncStatusCode } from 'teleport/services/integrations/oktaStatusTypes';

export function getDurationText(date: Date) {
  if (!date || date.getTime() <= 0) {
    return 'not recorded yet';
  }

  return formatDistanceStrict(new Date(date), new Date(), { addSuffix: true });
}

export const Panel = styled(Flex).attrs({
  p: 4,
  borderRadius: 3,
  flexDirection: 'column',
  gap: 3,
})<{ withBorder?: boolean }>`
  flex-basis: 100%;
  min-width: 0;
  background-color: ${props => props.theme.colors.levels.surface};
  border: ${p =>
    p.withBorder ? `1px solid ${p.theme.colors.spotBackground[2]}` : 'none'};
  box-shadow: ${p => p.theme.boxShadow[0]};
`;

export const PanelTitle = styled(Text)`
  font-size: ${p => p.theme.fontSizes[4]}px;
  font-weight: 500;
`;

export const InnerCard = styled(Box).attrs({
  p: 4,
  borderRadius: 3,
})`
  flex-basis: 100%;
  border: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

export const TextWithBorderBottom = styled(Text)`
  padding-bottom: ${p => p.theme.space[2]}px;
  border-bottom: 1px solid ${p => p.theme.colors.spotBackground[0]};
`;

export const LinkedInnerCard = styled(InnerCard)`
  color: inherit;
  text-decoration: none;
  &:hover {
    cursor: pointer;
    border: 1px solid ${p => p.theme.colors.spotBackground[2]};
  }
`;

export const FlexWrap = styled(Flex)`
  @media screen and (max-width: ${p => p.theme.breakpoints.tablet}) {
    flex-wrap: wrap;
  }
`;

const LightLabel = styled(Label)`
  border-radius: 999px;
  ${p =>
    p.kind === 'success' &&
    `background: ${p.theme.colors.interactive.tonal.success[1]};
color: ${p.theme.colors.success.hover};`}
`;

export function CustomLabel({ enabled }: { enabled: boolean }) {
  return (
    <LightLabel kind={enabled ? 'success' : 'secondary'}>
      <Flex alignItems="center" gap={1}>
        {enabled ? (
          <CircleCheck size="small" py="6px" />
        ) : (
          <CircleCross size="small" py="6px" />
        )}
        <span
          css={`
            @media screen and (max-width: ${p => p.theme.breakpoints.medium}) {
              display: none;
            }
          `}
        >
          {enabled ? 'Enabled' : 'Disabled'}
        </span>
      </Flex>
    </LightLabel>
  );
}

export const CenteredFlex = styled(Flex)`
  align-items: center;
  justify-content: space-between;
  gap: ${p => p.theme.space[2]}px;
  margin-bottom: ${p => p.theme.space[2]}px;
`;

export function ErrorTooltip({
  statusCode,
  lastFailed,
  error,
}: {
  statusCode: PluginOktaSyncStatusCode;
  lastFailed: Date;
  error: string;
}) {
  if (statusCode === PluginOktaSyncStatusCode.Error) {
    return (
      <IconTooltip kind="error">
        <Text>
          <b>Last Failed:</b> {getDurationText(lastFailed)}
        </Text>
        {/* pre-line required to respect string containing \n\t chars */}
        <Text css={{ whiteSpace: 'pre-line' }}>{error}</Text>
      </IconTooltip>
    );
  }
  return null;
}

const CustomMenuItem = styled(MenuItem)`
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: ${p => p.theme.space[2]}px;
`;

export const StatusAndOptions = ({
  enabled,
  disabled = false,
  setEnabled = undefined,
  canDisable = true,
  options = [],
}: {
  enabled: boolean;
  disabled?: boolean;
  setEnabled?: (arg0: boolean) => void;
  canDisable?: boolean;
  options?: {
    label: string;
    onClick: () => void;
    Icon?: ComponentType<IconProps>;
    disabled?: boolean;
  }[];
}) => {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement | null>(null);

  return (
    <Flex flexDirection="row" alignItems="center" gap={3}>
      <CustomLabel enabled={enabled} />
      {!disabled && (setEnabled || options?.length) && (
        <>
          <HoverTooltip tipContent="Options">
            <ButtonIcon
              onClick={() => setOpen(!open)}
              ref={anchorRef}
              style={{
                padding: '8px',
                margin: '-8px',
              }}
              aria-label="Options"
              aria-expanded={open}
            >
              <MoreVert size={16} mt="2px" />
            </ButtonIcon>
          </HoverTooltip>

          <Menu
            open={open}
            onClose={() => setOpen(false)}
            anchorEl={anchorRef.current}
            anchorOrigin={{
              vertical: 'top',
              horizontal: 'right',
            }}
            transformOrigin={{
              vertical: 'top',
              horizontal: 'right',
            }}
            popoverCss={() => `margin-top: 38px;`}
          >
            <ToggleButton
              enabled={enabled}
              setEnabled={setEnabled}
              canDisable={canDisable}
            />
            {options.map(({ label, onClick, Icon, disabled }) => (
              <CustomMenuItem
                key={label}
                onClick={() => !disabled && onClick()}
                disabled={disabled}
              >
                {Icon ? <Icon size="small" /> : null}
                {label}
              </CustomMenuItem>
            ))}
          </Menu>
        </>
      )}
    </Flex>
  );
};

const ToggleButton = ({
  canDisable,
  enabled,
  setEnabled,
}: {
  canDisable: boolean;
  enabled: boolean;
  setEnabled: (arg0: boolean) => void;
}) => {
  if (!setEnabled || (enabled && !canDisable)) {
    return null;
  }
  if (!enabled) {
    return (
      <CustomMenuItem onClick={() => setEnabled(!enabled)}>
        <CircleCheck size="small" />
        Enable
      </CustomMenuItem>
    );
  }
  return (
    <CustomMenuItem
      onClick={() => setEnabled(!enabled)}
      css={`
        &:hover,
        &:focus-visible {
          color: ${p => p.theme.colors.error.hover};
        }
      `}
    >
      <CircleCross size="small" />
      Disable
    </CustomMenuItem>
  );
};
