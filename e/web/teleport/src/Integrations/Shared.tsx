import type { ComponentProps, PropsWithChildren, ReactNode } from 'react';
import styled from 'styled-components';

import { Box, Flex, H2 } from 'design';
import { Info } from 'design/Icon';
import { HoverTooltip } from 'design/Tooltip';

const StyledBoxComponent = styled(Flex)`
  position: relative;
  background-color: ${props => props.theme.colors.levels.surface};
  box-shadow:
    0 2px 1px -1px rgba(0, 0, 0, 0.2),
    0 1px 1px 0 rgba(0, 0, 0, 0.14),
    0 1px 3px 0 rgba(0, 0, 0, 0.12);
`;

type StyledBoxProps =
  | {
      header: string;
      tooltip?: string | ReactNode;
    }
  | {
      header?: ReactNode;
      tooltip?: never;
    };

export const StyledBox = ({
  header,
  tooltip,
  children,
  ...props
}: PropsWithChildren<
  StyledBoxProps & ComponentProps<typeof StyledBoxComponent>
>) => {
  let Header: ReactNode;

  switch (typeof header) {
    case 'string':
      if (tooltip) {
        Header = (
          <Flex flexDirection="row" alignItems="center" gap={2} mt={-1}>
            <H2>{header}</H2>
            <HoverTooltip tipContent={tooltip}>
              <Info size="medium" />
            </HoverTooltip>
          </Flex>
        );
      } else {
        Header = <H2 mt={-1}>{header}</H2>;
      }
      break;
    case 'undefined':
      Header = null;
      break;
    default:
      Header = <Box mt={-1}>{header}</Box>;
      break;
  }

  return (
    <StyledBoxComponent
      p={4}
      gap={3}
      borderRadius={3}
      flexDirection="column"
      maxWidth="800px"
      {...props}
    >
      {Header}
      {children}
    </StyledBoxComponent>
  );
};
