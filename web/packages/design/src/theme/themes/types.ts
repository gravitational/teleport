/**
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

import { Fonts } from '../fonts';
import { blueGrey } from '../palette';
import typography, { fontSizes, fontWeights } from '../typography';

type InteractiveColorGroup = {
  default: string;
  hover: string;
  active: string;
};

export type ThemeColors = {
  /**
    Colors in `levels` are used to reflect the perceived depth of elements in the UI.
    The further back an element is, the more "sunken" it is, and the more forwards it is, the more "elevated" it is (think CSS z-index).

    A `sunken` color would be used to represent something like the background of the app.
    While `surface` would be the color of the primary surface where most content is located (such as tables).
    Any colors more "elevated" than that would be used for things such as popovers, menus, and dialogs.

    For more information on this concept: https://m3.material.io/styles/elevation/applying-elevation
   */
  levels: {
    deep: string;
    sunken: string;
    surface: string;
    elevated: string;
    popout: string;
  };

  /**
    Spot backgrounds are used as highlights, for example
    to indicate a hover or active state for an item in a menu.
    @deprecated Use `interactive.tonal.neutral` instead.
  */
  spotBackground: string[];

  brand: string;

  /**
   * Interactive colors are used to highlight different actions or states
   * based on intent.
   *
   * For example, primary would be used for as selected states.
   */
  interactive: {
    solid: {
      primary: InteractiveColorGroup;
      success: InteractiveColorGroup;
      accent: InteractiveColorGroup;
      danger: InteractiveColorGroup;
      alert: InteractiveColorGroup;
    };
    tonal: {
      primary: string[];
      neutral: string[];
      success: string[];
      danger: string[];
      alert: string[];
      informational: string[];
    };
  };

  text: {
    /** The most important text. */
    main: string;
    /** Slightly muted text. */
    slightlyMuted: string;
    /** Muted text. Also used as placeholder text in forms. */
    muted: string;
    /** Disabled text. */
    disabled: string;
    /**
      For text on  a background that is on a color opposite to the theme. For dark theme,
      this would mean text that is on a light background.
    */
    primaryInverse: string;
  };

  buttons: {
    text: string;
    textDisabled: string;
    bgDisabled: string;

    /** @deprecated Use `interactive.solid.primary` instead. */
    primary: {
      text: string;
      default: string;
      hover: string;
      active: string;
    };

    /** @deprecated Use `interactive.tonal.neutral` instead. */
    secondary: {
      default: string;
      hover: string;
      active: string;
    };

    border: {
      default: string;
      hover: string;
      active: string;
      border: string;
    };

    /** @deprecated Use `interactive.solid.danger` instead. */
    warning: {
      text: string;
      default: string;
      hover: string;
      active: string;
    };

    trashButton: { default: string; hover: string };

    /** @deprecated Use `interactive.solid.accent` instead. */
    link: {
      default: string;
      hover: string;
      active: string;
    };
  };

  tooltip: {
    background: string;
    inverseBackground: string;
  };

  progressBarColor: string;

  /** @deprecated Use `interactive.solid.danger` instead. */
  error: {
    main: string;
    hover: string;
    active: string;
  };

  /** @deprecated Use `interactive.solid.alert` instead. */
  warning: {
    main: string;
    hover: string;
    active: string;
  };

  /** @deprecated Use `interactive.solid.success` instead. */
  success: {
    main: string;
    hover: string;
    active: string;
  };

  /** @deprecated Use `interactive.solid.accent` instead. */
  accent: {
    main: string;
    hover: string;
    active: string;
  };

  notice: {
    background: string;
  };

  action: {
    active: string;
    hover: string;
    hoverOpacity: number;
    selected: string;
    disabled: string;
    disabledBackground: string;
  };

  terminal: {
    foreground: string;
    background: string;
    selectionBackground: string;
    cursor: string;
    cursorAccent: string;
    red: string;
    green: string;
    yellow: string;
    blue: string;
    magenta: string;
    cyan: string;
    brightWhite: string;
    white: string;
    brightBlack: string;
    black: string;
    brightRed: string;
    brightGreen: string;
    brightYellow: string;
    brightBlue: string;
    brightMagenta: string;
    brightCyan: string;
    searchMatch: string;
    activeSearchMatch: string;
  };

  editor: {
    abbey: string;
    purple: string;
    cyan: string;
    picton: string;
    sunflower: string;
    caribbean: string;
  };

  link: string;

  highlightedNavigationItem: string;

  dataVisualisation: DataVisualisationColors;
  accessGraph: AccessGraphColors;
} & SharedColors;

interface AccessGraphColors {
  dotsColor: string;
  nodes: {
    user: AccessGraphNodeColors;
    userGroup: AccessGraphNodeColors;
    resource: AccessGraphNodeColors;
    resourceGroup: AccessGraphNodeColors;
    allowedAction: AccessGraphNodeColors;
    disallowedAction: AccessGraphNodeColors;
    allowedRequest: AccessGraphNodeColors;
    disallowedRequest: AccessGraphNodeColors;
    allowedReview: AccessGraphNodeColors;
    disallowedReview: AccessGraphNodeColors;
    accessRequest: AccessGraphNodeColors;
    temporaryUserGroup: AccessGraphNodeColors;
    temporaryResourceGroup: AccessGraphNodeColors;
    temporaryAllowedAction: AccessGraphNodeColors;
  };
  edges: {
    dynamicMemberOf: AccessGraphEdgeColors;
    memberOf: AccessGraphEdgeColors;
    reverse: AccessGraphEdgeColors;
    allowed: AccessGraphEdgeColors;
    disallowed: AccessGraphEdgeColors;
    restricted: AccessGraphEdgeColors;
    default: AccessGraphEdgeColors;
    requestedBy: AccessGraphEdgeColors;
    requestedResource: AccessGraphEdgeColors;
    requestedAction: AccessGraphEdgeColors;
  };
}

interface AccessGraphNodeColors {
  background: string;
  borderColor: string;
  typeColor: string;
  iconBackground: string;
  handleColor: string;
  highlightColor: string;
  label: {
    background: string;
    color: string;
  };
}

interface AccessGraphEdgeColors {
  color: string;
  stroke: string;
}

export type SharedColors = {
  dark: string;
  light: string;
  interactionHandle: string;
  grey: typeof blueGrey;
  subtle: string;
  bgTerminal: string;
  highlight: string;
  disabled: string;
  info: string;
};

export type DataVisualisationColors = {
  primary: VisualisationColors;
  secondary: VisualisationColors;
  tertiary: VisualisationColors;
};

type VisualisationColors = {
  purple: string;
  wednesdays: string;
  picton: string;
  sunflower: string;
  caribbean: string;
  abbey: string;
  cyan: string;
};

export type SharedStyles = {
  sidebarWidth: number;
  boxShadow: string[];
  breakpoints: {
    mobile: number;
    tablet: number;
    desktop: number;
    small: number;
    medium: number;
    large: number;
  };
  topBarHeight: number[];
  space: number[];
  borders: (string | number)[];
  typography: typeof typography;
  font: string;
  fonts: Fonts;
  fontWeights: typeof fontWeights;
  fontSizes: typeof fontSizes;
  radii: (number | string)[];
  regular: number;
  bold: number;
};

export type Theme = {
  name: string;
  /** This field should be either `light` or `dark`. This is used to determine things like which version of logos to use
  so that they contrast properly with the background. */
  type: 'dark' | 'light';
  /** Whether this is a custom theme and not Dark Theme/Light Theme. */
  isCustomTheme: boolean;
  colors: ThemeColors;
} & SharedStyles;
