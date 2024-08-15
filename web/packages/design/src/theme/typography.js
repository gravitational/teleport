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

const light = 300;
const regular = 400;
const medium = 500;
const bold = 600;

export const fontSizes = [10, 12, 14, 16, 18, 20, 22, 24, 26, 28, 34];

export const fontWeights = { light, regular, bold };

const typography = {
  h1: {
    fontWeight: light,
    fontSize: '34px',
    lineHeight: '56px',
  },
  h2: {
    fontWeight: light,
    fontSize: '28px',
    lineHeight: '32px',
  },
  h3: {
    fontWeight: 300,
    fontSize: '22px',
    lineHeight: '32px',
  },
  h4: {
    fontWeight: regular,
    fontSize: '18px',
    lineHeight: '32px',
  },
  h5: {
    fontWeight: regular,
    fontSize: '16px',
    lineHeight: '24px',
  },
  h6: {
    fontWeight: bold,
    fontSize: '14px',
    lineHeight: '24px',
  },
  body1: {
    fontWeight: regular,
    fontSize: '14px',
    lineHeight: '24px',
  },
  body2: {
    fontWeight: regular,
    fontSize: '12px',
    lineHeight: '16px',
  },
  paragraph: {
    fontWeight: light,
    fontSize: '16px',
    lineHeight: '32px',
  },
  paragraph2: {
    fontWeight: light,
    fontSize: '12px',
    lineHeight: '24px',
  },
  subtitle1: {
    fontWeight: regular,
    fontSize: '14px',
    lineHeight: '24px',
  },
  subtitle2: {
    fontWeight: bold,
    fontSize: '10px',
    lineHeight: '16px',
  },
  subtitle3: {
    fontSize: '10px',
    fontWeight: regular,
    lineHeight: '14px',
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

  newBody1: {
    fontSize: '16px',
    fontWeight: light,
    lineHeight: '24px',
    letterSpacing: '0.08px',
  },
  newBody2: {
    fontSize: '14px',
    fontWeight: light,
    lineHeight: '24px',
    letterSpacing: '0.035px',
  },
  newBody3: {
    fontSize: '12px',
    fontWeight: regular,
    lineHeight: '20px',
    letterSpacing: '0.015px',
  },
  newBody4: {
    fontSize: '10px',
    fontWeight: regular,
    lineHeight: '16px',
    letterSpacing: '0.013px',
  },

  /**
   * Don't use directly, prefer the `H1` component except for text that doesn't
   * introduce document structure.
   */
  newH1: {
    fontWeight: medium,
    fontSize: '24px',
    lineHeight: '32px',
  },
  /**
   * Don't use directly, prefer the `H2` component except for text that doesn't
   * introduce document structure.
   */
  newH2: {
    fontWeight: medium,
    fontSize: '18px',
    lineHeight: '24px',
  },
  /**
   * Don't use directly, prefer the `H3` component except for text that doesn't
   * introduce document structure.
   */
  newH3: {
    fontWeight: bold,
    fontSize: '14px',
    lineHeight: '20px',
  },
  newH4: {
    fontWeight: medium,
    fontSize: '12px',
    lineHeight: '20px',
    letterSpacing: '0.03px',
    textTransform: 'uppercase',
  },
  newSubtitle1: {
    fontSize: '16px',
    fontWeight: regular,
    lineHeight: '24px',
    letterSpacing: '0.024px',
  },
  newSubtitle2: {
    fontSize: '14px',
    fontWeight: regular,
    lineHeight: '20px',
    letterSpacing: '0.014px',
  },
  newSubtitle3: {
    fontSize: '12px',
    fontWeight: bold,
    lineHeight: '20px',
    letterSpacing: '0.012px',
  },
};

export default typography;
