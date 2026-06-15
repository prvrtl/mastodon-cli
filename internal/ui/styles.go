package ui

import "github.com/charmbracelet/lipgloss"

var (
	colMasto   = lipgloss.Color("#6364ff")
	colMastoLt = lipgloss.Color("#858afa")
	colWhite   = lipgloss.Color("#ffffff")

	colAccent = colMasto
	colFg     = lipgloss.Color("#e6ebf2")
	colMuted  = lipgloss.Color("#8c95b0")
	colFaint  = lipgloss.Color("#606984")
	colDim    = lipgloss.Color("#2b2d3a")
	colBg     = lipgloss.Color("#191b22")
	colPanel  = lipgloss.Color("#282c37")

	colGreen   = lipgloss.Color("#79bd9a")
	colYellow  = lipgloss.Color("#e6a817")
	colRed     = lipgloss.Color("#df405a")
	colMagenta = lipgloss.Color("#a8a9ff")
)

var (
	styleAuthor = lipgloss.NewStyle().Foreground(colFg).Bold(true)
	styleHandle = lipgloss.NewStyle().Foreground(colMuted)
	styleTime   = lipgloss.NewStyle().Foreground(colFaint)
	styleIndex  = lipgloss.NewStyle().Foreground(colWhite).Background(colMasto).Bold(true)
	styleBody   = lipgloss.NewStyle().Foreground(colFg)
	styleMeta   = lipgloss.NewStyle().Foreground(colMuted)
	styleBoost  = lipgloss.NewStyle().Foreground(colGreen)
	styleNotif  = lipgloss.NewStyle().Foreground(colMagenta).Bold(true)
	styleSpoil  = lipgloss.NewStyle().Foreground(colYellow).Italic(true)

	styleSystem = lipgloss.NewStyle().Foreground(colMuted).Italic(true)
	styleErr    = lipgloss.NewStyle().Foreground(colRed).Bold(true)
	styleOK     = lipgloss.NewStyle().Foreground(colGreen).Bold(true)

	styleHeader     = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleStatusLine = lipgloss.NewStyle().Foreground(colMuted)

	stylePromptBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(0, 1)

	styleFav     = lipgloss.NewStyle().Foreground(colYellow)
	styleReblog  = lipgloss.NewStyle().Foreground(colGreen)
	styleReplies = lipgloss.NewStyle().Foreground(colMuted)

	styleSep = lipgloss.NewStyle().Foreground(colDim)

	styleMenuHint = lipgloss.NewStyle().Foreground(colFaint).Italic(true)
	styleMenuItem = lipgloss.NewStyle().Foreground(colFg)
	styleMenuSel  = lipgloss.NewStyle().Foreground(colWhite).Background(colMasto).Bold(true)
)
