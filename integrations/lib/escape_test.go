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

package lib

import "fmt"

func ExampleMarkdownEscape() {
	fmt.Printf("%q\n", MarkdownEscape("     ", 1000))
	fmt.Printf("%q\n", MarkdownEscape("abc", 1000))
	fmt.Printf("%q\n", MarkdownEscape("`foo` `bar`", 1000))
	fmt.Printf("%q\n", MarkdownEscape("  123456789012345  ", 10))

	// Output: "(empty)"
	// "```\nabc```"
	// "```\n`\ufefffoo`\ufeff `\ufeffbar`\ufeff```"
	// "```\n1234567890``` (truncated)"
}

func ExampleMarkdownEscapeInLine() {
	fmt.Printf("%q\n", MarkdownEscapeInLine("     ", 1000))
	fmt.Printf("%q\n", MarkdownEscapeInLine("abc", 1000))
	fmt.Printf("%q\n", MarkdownEscapeInLine("`foo` `bar`", 1000))
	fmt.Printf("%q\n", MarkdownEscapeInLine("  123456789012345  ", 10))

	// Output: "(empty)"
	// "`abc`"
	// "``\ufefffoo`\ufeff `\ufeffbar`\ufeff`"
	// "`1234567890` (truncated)"
}
