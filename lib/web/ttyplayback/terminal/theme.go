/**
 * Copyright (C) 2024 Gravitational, Inc.
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

package terminal

import (
	"fmt"
	"strconv"
	"strings"
)

type Theme struct {
	Background RGB8
	Foreground RGB8
	Colors     [256]RGB8
}

func parseHexColor(hex string) (RGB8, error) {
	if len(hex) != 6 {
		return RGB8{}, fmt.Errorf("hex color must be 6 characters, got %d", len(hex))
	}

	var r, g, b uint64
	var err error

	if r, err = parseHexByte(hex[0:2]); err != nil {
		return RGB8{}, err
	}
	if g, err = parseHexByte(hex[2:4]); err != nil {
		return RGB8{}, err
	}
	if b, err = parseHexByte(hex[4:6]); err != nil {
		return RGB8{}, err
	}

	return RGB8{uint8(r), uint8(g), uint8(b)}, nil
}

func parseHexByte(s string) (uint64, error) {
	return strconv.ParseUint(s, 16, 8)
}

var DraculaTheme = "282a36,f8f8f2,21222c,ff5555,50fa7b,f1fa8c,bd93f9,ff79c6,8be9fd,f8f8f2,6272a4,ff6e6e,69ff94,ffffa5,d6acff,ff92df,a4ffff,ffffff"

func ParseTheme(themeStr string) (*Theme, error) {
	colors := strings.Split(themeStr, ",")
	if len(colors) < 16 {
		return nil, fmt.Errorf("theme must have at least 16 colors, got %d", len(colors))
	}

	theme := &Theme{}

	// First color is background
	if bg, err := parseHexColor(colors[0]); err == nil {
		theme.Background = bg
	} else {
		return nil, fmt.Errorf("invalid background color: %w", err)
	}

	// Second color is foreground
	if fg, err := parseHexColor(colors[1]); err == nil {
		theme.Foreground = fg
	} else {
		return nil, fmt.Errorf("invalid foreground color: %w", err)
	}

	// Initialize all 256 colors with defaults
	for i := 0; i < 256; i++ {
		theme.Colors[i] = RGB8{0, 0, 0}
	}

	// Parse the 16 ANSI colors (skip first 2 which are bg/fg)
	for i := 0; i < 16 && i+2 < len(colors); i++ {
		if c, err := parseHexColor(colors[i+2]); err == nil {
			theme.Colors[i] = c
		} else {
			return nil, fmt.Errorf("invalid color at index %d: %w", i+2, err)
		}
	}

	// Generate colors 16-231 (6x6x6 color cube)
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				idx := 16 + r*36 + g*6 + b
				theme.Colors[idx] = RGB8{
					R: uint8(r * 51),
					G: uint8(g * 51),
					B: uint8(b * 51),
				}
			}
		}
	}

	// Generate colors 232-255 (grayscale)
	for i := 232; i < 256; i++ {
		gray := uint8((i-232)*10 + 8)
		theme.Colors[i] = RGB8{gray, gray, gray}
	}

	return theme, nil
}
