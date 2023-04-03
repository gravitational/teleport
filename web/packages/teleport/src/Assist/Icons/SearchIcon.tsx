import type { IconProps } from './types';

export function SearchIcon({ size = 24 }: IconProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      x="0px"
      y="0px"
      width={size}
      height={size}
      viewBox="0 0 24 24"
    >
      <path
        fill="white"
        d="M10 9A1 1 0 1 0 10 11 1 1 0 1 0 10 9zM7 9A1 1 0 1 0 7 11 1 1 0 1 0 7 9zM13 9A1 1 0 1 0 13 11 1 1 0 1 0 13 9zM22 20L20 22 16 18 16 16 18 16z"
      />
      <path
        fill="white"
        d="M10,18c-4.4,0-8-3.6-8-8c0-4.4,3.6-8,8-8c4.4,0,8,3.6,8,8C18,14.4,14.4,18,10,18z M10,4c-3.3,0-6,2.7-6,6c0,3.3,2.7,6,6,6 c3.3,0,6-2.7,6-6C16,6.7,13.3,4,10,4z"
      />
      <path
        fill="white"
        d="M15.7 14.5H16.7V18H15.7z"
        transform="rotate(-45.001 16.25 16.249)"
      />
    </svg>
  );
}
