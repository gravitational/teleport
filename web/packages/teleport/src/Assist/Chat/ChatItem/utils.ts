export function getBorderRadius(
  isTeleport: boolean,
  isFirst: boolean,
  isLast: boolean
) {
  if (isTeleport) {
    return `${isFirst ? '14px' : '5px'} 14px 14px ${isLast ? '14px' : '5px'}`;
  }

  return `14px ${isFirst ? '14px' : '5px'} ${isLast ? '14px' : '5px'} 14px`;
}
