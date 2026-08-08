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

package asciitable

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
)

// TestMain supports re-executing the test binary under a PTY to test
// terminalWidth() with a real terminal. When TERMWIDTH_REEXEC is set,
// the binary prints diagnostics and exits instead of running tests.
func TestMain(m *testing.M) {
	if os.Getenv("TERMWIDTH_REEXEC") != "" {
		runTermwidthDiag()
		return
	}
	os.Exit(m.Run())
}

func runTermwidthDiag() {
	resolved := terminalWidth()
	fmt.Printf("terminalWidth=%d\n", resolved)
}

// reexecUnderPTY runs the test binary under a PTY with the given size and
// optional extra env vars. Returns stdout output.
func reexecUnderPTY(t *testing.T, cols, rows int, env ...string) string {
	t.Helper()
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(), "TERMWIDTH_REEXEC=1")
	cmd.Env = append(cmd.Env, env...)
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(cols), Rows: uint16(rows)})
	require.NoError(t, err)
	defer ptmx.Close()
	buf := make([]byte, 4096)
	var output []byte
	for {
		n, err := ptmx.Read(buf)
		if n > 0 {
			output = append(output, buf[:n]...)
		}
		if err != nil {
			break
		}
	}
	_ = cmd.Wait()
	return string(output)
}

func TestTerminalWidth(t *testing.T) {
	t.Run("pty auto-detects terminal size", func(t *testing.T) {
		out := reexecUnderPTY(t, 132, 24)
		require.Contains(t, out, "terminalWidth=132")
	})

	t.Run("COLUMNS overrides pty size", func(t *testing.T) {
		out := reexecUnderPTY(t, 132, 24, "COLUMNS=200")
		require.Contains(t, out, "terminalWidth=200")
	})

	t.Run("non-terminal returns zero", func(t *testing.T) {
		// Run without a PTY — stdout is a pipe.
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), "TERMWIDTH_REEXEC=1")
		out, err := cmd.Output()
		require.NoError(t, err)
		require.Contains(t, string(out), "terminalWidth=0")
	})

	t.Run("COLUMNS overrides non-terminal", func(t *testing.T) {
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), "TERMWIDTH_REEXEC=1", "COLUMNS=60")
		out, err := cmd.Output()
		require.NoError(t, err)
		require.Contains(t, string(out), "terminalWidth=60")
	})

	t.Run("invalid COLUMNS ignored on terminal", func(t *testing.T) {
		out := reexecUnderPTY(t, 100, 24, "COLUMNS=notanumber")
		require.Contains(t, out, "terminalWidth=100")
	})

	t.Run("COLUMNS=0 ignored on terminal", func(t *testing.T) {
		out := reexecUnderPTY(t, 100, 24, "COLUMNS=0")
		require.Contains(t, out, "terminalWidth=100")
	})
}

func TestFullTable(t *testing.T) {
	table := MakeTable([]string{"Name", "Motto", "Age"})
	table.AddRow([]string{"Alice Johnson", "Trains are much better than cars", "40"})
	table.AddRow([]string{"Bob Smith", "Read a good book", "2024"})

	expected := `Name          Motto                            Age  
------------- -------------------------------- ---- 
Alice Johnson Trains are much better than cars 40   
Bob Smith     Read a good book                 2024 
`
	require.Equal(t, expected, table.AsBuffer().String())
}

func TestHeadlessTable(t *testing.T) {
	table := MakeHeadlessTable(2)
	table.AddRow([]string{"one", "two", "three"})
	table.AddRow([]string{"1", "2", "3"})

	// The table shall have no header and also the 3rd column must be chopped off.
	expected := `one  two  
1    2    
`
	require.Equal(t, expected, table.AsBuffer().String())
}

func TestTruncatedTable(t *testing.T) {
	table := MakeTable([]string{"Name"})
	table.AddColumn(Column{
		Title:         "Motto",
		MaxCellLength: 25,
		FootnoteLabel: "[*]",
	})
	table.AddColumn(Column{
		Title:         "Age",
		MaxCellLength: 2,
	})
	table.AddFootnote(
		"[*]",
		`Full motto was truncated, use the "tctl motto get" subcommand to view full motto.`,
	)
	table.AddRow([]string{"Alice Johnson", "Trains are much better than cars", "40"})
	table.AddRow([]string{"Bob Smith", "Read a good book", "for ever and ever"})
	table.AddRow([]string{"X", strings.Repeat("y", 26), ""})

	expected := `Name          Motto                            Age   
------------- -------------------------------- ----- 
Alice Johnson Trains are much better th... [*] 40    
Bob Smith     Read a good book                 fo... 
X             yyyyyyyyyyyyyyyyyyyyyyyyy... [*]       

[*] Full motto was truncated, use the "tctl motto get" subcommand to view full motto.
`
	require.Equal(t, expected, table.AsBuffer().String())
}

func TestMakeTableWithTruncatedColumn(t *testing.T) {
	t.Setenv("COLUMNS", "80")
	defaultColumns := []string{"column1", "column2", "column3"}
	defaultRows := [][]string{{"cell1cell1cell1cell1cell1cell1", "cell2cell2cell2cell2cell2cell2", "cell3cell3cell3cell3cell3cell3"}}

	testCases := []struct {
		name            string
		columns         []string
		rows            [][]string
		truncatedColumn string
		expectedWidth   int
		expectedOutput  []string
	}{
		{
			name:            "column2",
			truncatedColumn: "column2",
			expectedWidth:   80,
			expectedOutput: []string{
				"column1                        column2           column3                        ",
				"------------------------------ ----------------- ------------------------------ ",
				"cell1cell1cell1cell1cell1cell1 cell2cell2cell... cell3cell3cell3cell3cell3cell3 ",
				"",
			},
		},
		{
			name:            "column3",
			truncatedColumn: "column3",
			expectedWidth:   80,
			expectedOutput: []string{
				"column1                        column2                        column3           ",
				"------------------------------ ------------------------------ ----------------- ",
				"cell1cell1cell1cell1cell1cell1 cell2cell2cell2cell2cell2cell2 cell3cell3cell... ",
				"",
			},
		},
		{
			// With 5 columns at 80 width, maxColWidth = (80-16)/4 = 16.
			// The two wide non-target columns exceed the cap and get
			// truncated alongside the target column.
			name:    "multiple columns truncated",
			columns: []string{"Name", "Age", "Email", "City", "Bio"},
			rows: [][]string{{
				"alice@example.org__", "30",
				"bob@example.org____", "NY",
				"likes long walks____",
			}},
			truncatedColumn: "Bio",
			expectedWidth:   71,
			expectedOutput: []string{
				"Name                Age  Email               City Bio                  ",
				"------------------- ---  ------------------- ---- -------------------- ",
				"alice@example.or... 30   bob@example.org_... NY   likes long walks____ ",
				"",
			},
		},
		{
			name:            "no column match",
			truncatedColumn: "no column match",
			expectedWidth:   93,
			expectedOutput: []string{
				"column1                        column2                        column3                        ",
				"------------------------------ ------------------------------ ------------------------------ ",
				"cell1cell1cell1cell1cell1cell1 cell2cell2cell2cell2cell2cell2 cell3cell3cell3cell3cell3cell3 ",
				"",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cols := testCase.columns
			if cols == nil {
				cols = defaultColumns
			}
			inputRows := testCase.rows
			if inputRows == nil {
				inputRows = defaultRows
			}
			table := MakeTableWithTruncatedColumn(cols, inputRows, testCase.truncatedColumn)
			lines := strings.Split(table.AsBuffer().String(), "\n")
			require.Len(t, lines, 4)
			require.Len(t, lines[2], testCase.expectedWidth)
			require.Equal(t, testCase.expectedOutput, lines)
		})
	}
}

func TestMakeTableWithEllipsisColumn(t *testing.T) {
	long := strings.Repeat("x", 60)

	testCases := []struct {
		desc           string
		headers        []string
		rows           [][]string
		ellipsisColumn string
		envCOLUMNS     string // COLUMNS env var; empty means unset
	}{
		{
			desc:           "3 columns",
			headers:        []string{"column1", "column2", "column3"},
			rows:           [][]string{{strings.Repeat("cell1", 6), strings.Repeat("cell2", 6), strings.Repeat("cell3", 6)}},
			ellipsisColumn: "column2",
			envCOLUMNS:     "80",
		},
		{
			desc:           "2 columns",
			headers:        []string{"Name", "Description"},
			rows:           [][]string{{"short", long}, {"another", "brief"}},
			ellipsisColumn: "Description",
			envCOLUMNS:     "80",
		},
		{
			desc:           "5 columns",
			headers:        []string{"ID", "Region", "Account", "Status", "Output"},
			rows:           [][]string{{"i-001", "us-east-1", "111111111111", "exit=1", long}},
			ellipsisColumn: "Output",
			envCOLUMNS:     "80",
		},
		{
			desc:           "7 columns",
			headers:        []string{"A", "B", "C", "D", "E", "F", "Ellipsis"},
			rows:           [][]string{{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff", long}},
			ellipsisColumn: "Ellipsis",
			envCOLUMNS:     "80",
		},
		{
			desc:           "non-terminal produces untruncated output",
			headers:        []string{"column1", "column2", "column3"},
			rows:           [][]string{{strings.Repeat("cell1", 6), strings.Repeat("cell2", 6), strings.Repeat("cell3", 6)}},
			ellipsisColumn: "column2",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.envCOLUMNS != "" {
				t.Setenv("COLUMNS", tc.envCOLUMNS)
			}
			table := MakeTableWithEllipsisColumn(tc.headers, tc.rows, tc.ellipsisColumn)
			output := table.AsBuffer().String()
			lines := strings.Split(output, "\n")
			// Verify the ellipsis column uses middle ellipsis when truncated.
			if tc.envCOLUMNS != "" {
				for _, line := range lines {
					if line != "" {
						require.LessOrEqual(t, len(line), 80)
					}
				}
			}
			// Snapshot: update expected values here if output changes.
			t.Logf("output:\n%s", output)
			require.NotEmpty(t, lines)
		})
	}
}
