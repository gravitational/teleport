import { ComponentType, useRef, useState } from 'react';
import styled from 'styled-components';

import { ButtonIcon, Flex, Label, Menu, MenuItem, Text } from 'design';
import { CircleCheck, CircleCross, MoreVert } from 'design/Icon';
import { IconProps } from 'design/Icon/Icon';
import { HoverTooltip } from 'design/Tooltip';

import { IntegrationLike } from 'teleport/Integrations/IntegrationList';
import {
  getStatusCodeDescription,
  getStatusCodeTitle,
  IntegrationStatusCode,
} from 'teleport/services/integrations';

export const OverallStatus = ({
  statusCode,
  status = undefined,
}: {
  statusCode: IntegrationStatusCode;
  status?: IntegrationLike['status'];
}) => {
  const statusDescription = status?.errorMessage
    ? getStatusCodeDescription(statusCode, status.errorMessage)
    : undefined;

  return (
    <HoverTooltip tipContent={statusDescription}>
      <Label kind={getLabelKind(statusCode)}>
        <Text>{getStatusCodeTitle(statusCode)}</Text>
      </Label>
    </HoverTooltip>
  );
};

export const getLabelKind = (statusCode: IntegrationStatusCode) => {
  switch (statusCode) {
    case IntegrationStatusCode.Unknown:
      return 'warning';
    case IntegrationStatusCode.Running:
      return 'success';
    case IntegrationStatusCode.SlackNotInChannel:
      return 'warning';
    case IntegrationStatusCode.Draft:
      return 'warning';
    case IntegrationStatusCode.OktaConfigError:
    default:
      // default to error kind
      return 'danger';
  }
};

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
    tooltip?: string;
  }[];
}) => {
  const [open, setOpen] = useState(false);
  const anchorRef = useRef<HTMLButtonElement | null>(null);

  if (!setEnabled && options?.length === 0) {
    return <CustomLabel enabled={enabled} />;
  }

  return (
    <Flex flexDirection="row" alignItems="center" gap={3}>
      <CustomLabel enabled={enabled} />
      {!disabled && (
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
            {options.map(({ label, onClick, Icon, disabled, tooltip }) => (
              <CustomMenuItem
                key={label}
                onClick={() => !disabled && onClick()}
                disabled={disabled}
              >
                {Icon ? <Icon size="small" /> : null}
                <HoverTooltip tipContent={tooltip}>
                  <Text>{label}</Text>
                </HoverTooltip>
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

const LightLabel = styled(Label)`
  border-radius: 999px;
  ${p =>
    p.kind === 'success' &&
    `background: ${p.theme.colors.interactive.tonal.success[1]};
color: ${p.theme.colors.success.hover};`}
`;

export function CustomLabel({ enabled }: { enabled: boolean }) {
  return (
    <Flex>
      <LightLabel kind={enabled ? 'success' : 'secondary'}>
        <Flex alignItems="center" gap={1}>
          {enabled ? (
            <CircleCheck size="small" py="6px" />
          ) : (
            <CircleCross size="small" py="6px" />
          )}
          <span
            css={`
              @media screen and (max-width: ${p =>
                  p.theme.breakpoints.medium}) {
                display: none;
              }
            `}
          >
            {enabled ? 'Enabled' : 'Disabled'}
          </span>
        </Flex>
      </LightLabel>
    </Flex>
  );
}
