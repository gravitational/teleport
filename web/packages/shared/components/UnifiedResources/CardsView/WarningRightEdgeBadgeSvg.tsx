/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

/**
 * This svg is different from existing warning svg's in that it's specifically
 * tailored to fit the design of unified ResourceCard.tsx to the right edge
 * of the component.
 */
export function WarningRightEdgeBadgeSvg() {
  return (
    <svg
      /**
       * "className" is used instead of "fill" to style (color) the svg.
       * The svg needs to change style when user hovers anywhere in the
       * container the svg is a part of. This required the use of "className"
       * to target the svg as part of a container.
       */
      className="resource-health-status-svg"
      width="32"
      height="102"
      viewBox="0 0 32 102"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      preserveAspectRatio="none"
    >
      <path d="M24 0H0V2C3.31371 2 6 4.68629 6 8H32C32 3.58172 28.4183 0 24 0Z" />
      <path d="M6 7H32V95H6V8Z" />
      <path d="M32 94H6C6 97.3137 3.31371 100 0 100V102H24C28.4183 102 32 98.4183 32 94Z" />
      <path
        d="M19 49.0007C19.2761 49.0007 19.5 49.2245 19.5 49.5007V52.0007C19.5 52.2768 19.2761 52.5007 19 52.5007C18.7239 52.5007 18.5 52.2768 18.5 52.0007V49.5007C18.5 49.2245 18.7239 49.0007 19 49.0007Z"
        fill="black"
      />
      <path
        d="M19 54.8757C19.3452 54.8757 19.625 54.5958 19.625 54.2507C19.625 53.9055 19.3452 53.6257 19 53.6257C18.6548 53.6257 18.375 53.9055 18.375 54.2507C18.375 54.5958 18.6548 54.8757 19 54.8757Z"
        fill="black"
      />
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M17.7267 45.2292L12.254 54.8064C11.6953 55.7842 12.4013 57.0007 13.5274 57.0007H24.4728C25.5989 57.0007 26.3049 55.7842 25.7462 54.8064L20.2735 45.2292C19.7105 44.2439 18.2897 44.2439 17.7267 45.2292ZM13.1222 55.3025L18.5949 45.7254C18.7741 45.4119 19.2261 45.4119 19.4053 45.7254L24.8779 55.3025C25.0557 55.6136 24.8311 56.0007 24.4728 56.0007H13.5274C13.1691 56.0007 12.9445 55.6136 13.1222 55.3025Z"
        fill="black"
      />
    </svg>
  );
}
