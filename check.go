// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"text/tabwriter"
)

// CheckStatus is the outcome of one host check.
type CheckStatus int

// The statuses of a CheckResult.
const (
	// CheckOK means the requirement is met.
	CheckOK CheckStatus = iota
	// CheckWarn means the agent can start, but something will fail or be
	// slower later, such as a pool that does not fit in the host's memory.
	CheckWarn
	// CheckFail means the agent cannot run VMs; it refuses to start.
	CheckFail
)

func (s CheckStatus) String() string {
	switch s {
	case CheckOK:
		return "ok"
	case CheckWarn:
		return "warn"
	case CheckFail:
		return "FAIL"
	default:
		return fmt.Sprintf("status(%d)", int(s))
	}
}

// CheckResult is one check's outcome. Detail says what was found and, for
// CheckWarn and CheckFail, what to do about it, in one line.
type CheckResult struct {
	Check  string
	Status CheckStatus
	Detail string
}

// CheckReport is the rows a provider's host checks return, in the order the
// checks ran, which the agent prints with its own.
type CheckReport []CheckResult

// Failed reports whether any check failed.
func (r CheckReport) Failed() bool {
	for _, res := range r {
		if res.Status == CheckFail {
			return true
		}
	}
	return false
}

// Err joins the failed checks into one error, or returns nil.
func (r CheckReport) Err() error {
	var errs []error
	for _, res := range r {
		if res.Status == CheckFail {
			errs = append(errs, fmt.Errorf("%s: %s", res.Check, res.Detail))
		}
	}
	return errors.Join(errs...)
}

// WriteTable writes the report as a table with a header row.
func (r CheckReport) WriteTable(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "CHECK\tSTATUS\tDETAIL")
	for _, res := range r {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", res.Check, res.Status, oneLine(res.Detail))
	}
	return tw.Flush()
}

// Log writes one record per result: info for ok, warn for warn and error
// for a failure.
func (r CheckReport) Log(log *slog.Logger) {
	for _, res := range r {
		level := slog.LevelInfo
		switch res.Status {
		case CheckWarn:
			level = slog.LevelWarn
		case CheckFail:
			level = slog.LevelError
		default:
		}
		log.Log(context.Background(), level, "preflight", "check", res.Check, "status", res.Status.String(), "detail", oneLine(res.Detail))
	}
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
