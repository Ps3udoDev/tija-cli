package render

import "github.com/Ps3udoDev/tija-cli/internal/engine"

// Doctor renders each doctor check with its symbol and, when it did not
// pass, the hint to resolve it.
func Doctor(p Printer, r engine.DoctorReport) {
	if p.Mode == JSON {
		_ = p.JSONOut(r)
		return
	}
	if r.Root != "" {
		p.Field("Repository", r.Root)
		p.Line("")
	}
	for _, c := range r.Checks {
		msg := c.Title
		if c.Hint != "" && c.Status != engine.StatusOK {
			msg += " — " + c.Hint
		}
		switch c.Status {
		case engine.StatusOK:
			p.Ok(msg)
		case engine.StatusWarn:
			p.Warn(msg)
		default:
			p.Fail(msg)
		}
	}
}
