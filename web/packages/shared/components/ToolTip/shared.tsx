import { Origin } from 'design/Popover';
import { Position } from 'design/Popover/Popover';

export const anchorOriginForPosition = (pos: Position): Origin => {
  switch (pos) {
    case 'top':
      return { horizontal: 'center', vertical: 'top' };
    case 'right':
      return { horizontal: 'right', vertical: 'center' };
    case 'bottom':
      return { horizontal: 'center', vertical: 'bottom' };
    case 'left':
      return { horizontal: 'left', vertical: 'center' };
  }
};

export const transformOriginForPosition = (pos: Position): Origin => {
  switch (pos) {
    case 'top':
      return { horizontal: 'center', vertical: 'bottom' };
    case 'right':
      return { horizontal: 'left', vertical: 'center' };
    case 'bottom':
      return { horizontal: 'center', vertical: 'top' };
    case 'left':
      return { horizontal: 'right', vertical: 'center' };
  }
};
