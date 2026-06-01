package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type ConfirmationDialog struct {
	width       int
	question    string
	visible     bool
	blendOffset int
}

func NewConfirmationDialog() *ConfirmationDialog {
	return &ConfirmationDialog{
		width:       0,
		question:    "do you know what you're doing?",
		visible:     false,
		blendOffset: 0,
	}
}

func (c *ConfirmationDialog) View() string {
	return c.getContent()
}

func (c *ConfirmationDialog) getContent() string {
	// Define your start and end colors
	color1 := lipgloss.Color("#7D56F4")
	color2 := lipgloss.Color("#FF416C")

	style := lipgloss.
		NewStyle().
		BorderStyle(
			lipgloss.ThickBorder()).
		Width(c.width).
		BorderForegroundBlend(color1, color2).
		BorderForegroundBlendOffset(c.blendOffset)

	s := strings.Builder{}
	s.WriteString(c.question)
	s.WriteString("\n\n")
	s.WriteString("1.\tYes\n")
	s.WriteString("2.\tNo\n")
	return style.Render(s.String())
}

func (c *ConfirmationDialog) Update() {
	c.blendOffset++
}

func (c *ConfirmationDialog) SetWidth(width int) {
	c.width = width
}

func (c *ConfirmationDialog) Width() int {
	return c.width
}

func (c *ConfirmationDialog) SetQuestion(question string) {
	c.question = question
}

func (c *ConfirmationDialog) SetVisible(visibility bool) {
	c.visible = visibility
}

func (c *ConfirmationDialog) Visible() bool {
	return c.visible
}

func (c *ConfirmationDialog) GetHeight() int {
	if !c.visible {
		return 0
	}
	return lipgloss.Height(c.getContent())
}
