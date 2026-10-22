package render

import "github.com/Ps3udoDev/tija-cli/internal/engine"

// Doctor renders the detected project, then each doctor check with its
// symbol and, when it did not pass, the hint to resolve it.
func Doctor(p Printer, r engine.DoctorReport) {
	if p.Mode == JSON {
		if r.Agents == nil {
			r.Agents = []string{}
		}
		_ = p.JSONOut(r)
		return
	}
	if r.Detection != nil {
		p.Heading("Detected project")
		p.Line("")
	}
	if r.Root != "" {
		p.Field("Repository", r.Root)
	}
	if r.Detection != nil {
		detection(p, *r.Detection)
	}
	if r.Root != "" || r.Detection != nil {
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
