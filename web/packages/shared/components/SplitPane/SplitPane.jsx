/*
Copyright 2019 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import React from 'react';
import styled from 'styled-components';
import { Flex, Box } from 'design';

import useDraggable from 'shared/hooks/useDraggable';

export default function SplitPane({
  children,
  defaultSize,
  split = SplitEnum.VERTICAL,
  ...props
}) {
  const { onMouseDown, isDragging, position } = useDraggable();
  const Holder = split === SplitEnum.VERTICAL ? YHolder : XHolder;
  const hasFirstSide = !!children[0];
  const hasSecondSide = !!children[1];
  const hasTwoSides = hasFirstSide && hasSecondSide;

  return (
    <Pane split={split} {...props}>
      {hasFirstSide && (
        <PaneSide
          isDragging={isDragging}
          position={position}
          split={split}
          defaultSize={defaultSize}
          hasTwoSides={hasTwoSides}
        >
          {children[0]}
        </PaneSide>
      )}
      {hasTwoSides && (
        <Holder bg="levels.surfaceSecondary" onMouseDown={onMouseDown} />
      )}
      {hasSecondSide && <Flex flex="1 1 0%">{children[1]}</Flex>}
    </Pane>
  );
}

const Pane = styled(Flex)`
  ${props => {
    return {
      flexDirection: props.split === SplitEnum.VERTICAL ? 'row' : 'column',
      height: '100%',
    };
  }}
`;

export function PaneSide(props) {
  const { children, position, isDragging, split, defaultSize, hasTwoSides } =
    props;

  const compRef = React.useRef();

  // size contains the width and height of the element to be resized
  const size = React.useMemo(() => {
    return {
      height: 0,
      width: 0,
    };
  }, []);

  // initialSizeProps contains initial values for width and height
  const initialSizeProps = React.useMemo(
    () => getInitialSizeValues(split, defaultSize, hasTwoSides),
    [hasTwoSides, split]
  );

  // remember the element size before and after drag
  React.useEffect(() => {
    const element = compRef.current;
    size.width = element.clientWidth;
    size.height = element.clientHeight;
    // trigger windows resize event so other components can adjust
    // to the new div size
    window.dispatchEvent(new Event('resize'));
  }, [isDragging]);

  // reset width and height when nothing to split
  React.useEffect(() => {
    if (!hasTwoSides) {
      compRef.current.style.width = null;
      compRef.current.style.height = null;
      return;
    }

    if (split === SplitEnum.VERTICAL) {
      compRef.current.style.height = null;
    } else {
      compRef.current.style.width = null;
    }
  }, [hasTwoSides, split]);

  // handle drag movements which causes a position to change
  React.useEffect(() => {
    if (!isDragging) {
      return;
    }

    const element = compRef.current;
    const newWidth = size.width + position.x;
    const newHeight = size.height + position.y;
    if (split === SplitEnum.VERTICAL) {
      element.style.width = `${newWidth}px`;
    } else {
      element.style.height = `${newHeight}px`;
    }
  }, [position.x, position.y]);

  return (
    <Flex ref={compRef} {...initialSizeProps}>
      {children}
    </Flex>
  );
}

export const YHolder = styled(Box)`
  cursor: col-resize;
  width: 4px;
  height: 100%;
`;

export const XHolder = styled(Box)`
  cursor: row-resize;
  width: 100%;
  height: 4px;
`;

function getInitialSizeValues(split, defaultSize, hasTwoSides) {
  if (!hasTwoSides) {
    return {
      width: 'inherit',
      height: 'inherit',
      flex: '1',
    };
  }

  if (split === 'vertical') {
    return {
      width: defaultSize,
    };
  }

  return {
    height: defaultSize,
  };
}

const SplitEnum = {
  VERTICAL: 'vertical',
};
