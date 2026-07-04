package output

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
)

// Quiet, when true, suppresses all rendered output.
var Quiet bool

// Table writes a tab-aligned table. (No ANSI color is emitted, so NO_COLOR /
// non-TTY needs no special handling; kept simple and pipe-safe.)
func Table(w io.Writer, headers []string, rows [][]string) {
	if Quiet {
		return
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for i, h := range headers {
		if i > 0 {
			_, _ = fmt.Fprint(tw, "\t")
		}
		_, _ = fmt.Fprint(tw, h)
	}
	_, _ = fmt.Fprintln(tw)
	for _, row := range rows {
		for i, c := range row {
			if i > 0 {
				_, _ = fmt.Fprint(tw, "\t")
			}
			_, _ = fmt.Fprint(tw, c)
		}
		_, _ = fmt.Fprintln(tw)
	}
	_ = tw.Flush()
}

// JSON writes v as indented JSON.
func JSON(w io.Writer, v any) error {
	if Quiet {
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
