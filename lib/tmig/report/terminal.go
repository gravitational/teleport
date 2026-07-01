package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/teleport/lib/tmig/classify"
)

// RenderTerminal writes a human-readable summary block to a terminal.
func RenderTerminal(rpt *Report, w io.Writer) error {
	fmt.Fprintf(w, "SOURCE %s (%s) -> TARGET %s (%s)",
		rpt.Source.Name, rpt.Source.Version, rpt.Target.Name, rpt.Target.Version)
	if rpt.ScopesEnabled {
		fmt.Fprintf(w, "; scopes enabled OK")
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "identities: source=%s", rpt.Source.User)
	if rpt.Target.ScopePinned {
		fmt.Fprintf(w, ", target=%s (scope-pinned)", rpt.Target.User)
	} else {
		fmt.Fprintf(w, ", target=%s", rpt.Target.User)
	}
	fmt.Fprintln(w)

	if rpt.Scopable.Roles != nil {
		var parts []string
		for _, role := range []string{"Node", "Kube", "App", "Db"} {
			if rpt.Scopable.Roles[role] {
				parts = append(parts, role+" OK")
			} else {
				parts = append(parts, role+" NO")
			}
		}
		fmt.Fprintf(w, "roles scopable on TARGET: %s\n", strings.Join(parts, "  "))
	}

	fmt.Fprintf(w, "verdicts: AUTO %d | PREREQ %d",
		rpt.Summary.ByVerdict[classify.VerdictAuto],
		rpt.Summary.ByVerdict[classify.VerdictPrereq])
	if rpt.Summary.Blocked > 0 {
		fmt.Fprintf(w, " (%d blocked)", rpt.Summary.Blocked)
	}
	fmt.Fprintf(w, " | PIPELINE %d | MANUAL %d",
		rpt.Summary.ByVerdict[classify.VerdictPipeline],
		rpt.Summary.ByVerdict[classify.VerdictManual])
	if rpt.Summary.Orphans > 0 {
		fmt.Fprintf(w, " | ORPHAN %d", rpt.Summary.Orphans)
	}
	fmt.Fprintln(w)

	attn := rpt.Summary.Attention
	fmt.Fprintf(w, "%d hosts automatic", attn.AutomaticHosts)
	if attn.IaCActions > 0 {
		fmt.Fprintf(w, " | %d hosts unblocked by %d IaC applies", attn.IaCHostsCovered, attn.IaCActions)
	}
	if attn.PipelineActions > 0 {
		fmt.Fprintf(w, " | %d hosts covered by %d pipeline migrations", attn.PipelineHostsCovered, attn.PipelineActions)
	}
	if attn.ManualHosts > 0 {
		fmt.Fprintf(w, " | %d hosts need per-host manual steps", attn.ManualHosts)
	}
	fmt.Fprintln(w)

	for _, warn := range rpt.Warnings {
		fmt.Fprintf(w, "WARNING: %s\n", warn)
	}

	return nil
}
