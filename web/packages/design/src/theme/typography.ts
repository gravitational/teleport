/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

import React from 'react';

const light = 300;
const regular = 400;
const medium = 500;
const bold = 600;

export const fontSizes = [10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 34];

export const fontWeights = { light, regular, bold };

const typography = {
  body1: {
    fontSize: '16px',
    fontWeight: light,
    lineHeight: '24px',
    letterSpacing: '0.08px',
  },
  body2: {
    fontSize: '14px',
    fontWeight: light,
    lineHeight: '24px',
    letterSpacing: '0.035px',
  },
  body3: {
    fontSize: '12px',
    fontWeight: regular,
    lineHeight: '20px',
    letterSpacing: '0.015px',
  },
  body4: {
    fontSize: '10px',
    fontWeight: regular,
    lineHeight: '16px',
    letterSpacing: '0.013px',
  },

  /**
   * Don't use directly, prefer the `H1` component except for text that doesn't
   * introduce document structure.
   */
  h1: {
    fontWeight: medium,
    fontSize: '24px',
    lineHeight: '32px',
  },
  /**
   * Don't use directly, prefer the `H2` component except for text that doesn't
   * introduce document structure.
   */
  h2: {
    fontWeight: medium,
    fontSize: '18px',
    lineHeight: '24px',
  },
  /**
   * Don't use directly, prefer the `H3` component except for text that doesn't
   * introduce document structure.
   */
  h3: {
    fontWeight: bold,
    fontSize: '14px',
    lineHeight: '20px',
  },
  h4: {
    fontWeight: medium,
    fontSize: '12px',
    lineHeight: '20px',
    letterSpacing: '0.03px',
    textTransform: 'uppercase',
  },
  subtitle1: {
    fontSize: '16px',
    fontWeight: regular,
    lineHeight: '24px',
    letterSpacing: '0.024px',
  },
  subtitle2: {
    fontSize: '14px',
    fontWeight: regular,
    lineHeight: '20px',
    letterSpacing: '0.014px',
  },
  subtitle3: {
    fontSize: '12px',
    fontWeight: bold,
    lineHeight: '20px',
    letterSpacing: '0.012px',
  },
  table: {
    fontWeight: light,
    fontSize: '14px',
    lineHeight: '20px',
    letterSpacing: '0.035px',
  },
  dropdownTitle: {
    fontWeight: bold,
    fontSize: '14px',
    lineHeight: '20px',
  },

  // Declare value type, while the key type is narrowed down automatically.
} satisfies Record<string, React.CSSProperties>;

export default typography;
