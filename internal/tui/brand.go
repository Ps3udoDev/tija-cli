package tui

// The wordmark and a single colored mascot form the startup identity.
const logoBlock = `█████ ███    ██  ███
  █    █     ██ █   █
  █    █     ██ █████
  █    █  █  ██ █   █
  █   ███  ███  █   █`

const brandTagline = "Small cat. Big workflows."

func brandHeader(width int) string {
	if width < wideLayout {
		return logoStyle.Render("TIJA")
	}
	return logoStyle.Render(logoBlock)
}
