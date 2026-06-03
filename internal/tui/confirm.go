package tui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
)

type DialogModel struct {
	width       int
	question    string
	visible     bool
	blendOffset int
}

func InitialDialogModel() DialogModel {
	return DialogModel{
		width:       0,
		question:    "do you know what you're doing?",
		visible:     false,
		blendOffset: 0,
	}
}

func tickEvery() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (c DialogModel) Update(msg tea.Msg) (DialogModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return c.handleKeyPress(msg)
	case ConfirmationRequest:
		return c.handleConfirmationRequest(msg)
	case tea.WindowSizeMsg:
		return c.handleWindowResize(msg)
	case TickMsg:
		return c.handleTick(msg)
	}
	return c, nil
}

func (c DialogModel) handleDecision(msg DecisionMessage) (DialogModel, tea.Cmd) {
	c.visible = false
	return c, nil
}

func (c DialogModel) handleKeyPress(msg tea.KeyPressMsg) (DialogModel, tea.Cmd) {
	if !c.visible {
		return c, nil
	}

	switch msg.String() {
	case "1": //Yes
		return c, func() tea.Msg { return DecisionMessage{Approved: true} }
	case "2": //No
		return c, func() tea.Msg { return DecisionMessage{Approved: false} }
	}

	return c, nil
}

func (c DialogModel) handleTick(_ TickMsg) (DialogModel, tea.Cmd) {
	c.blendOffset++
	return c, tickEvery()
}

func (c DialogModel) handleWindowResize(msg tea.WindowSizeMsg) (DialogModel, tea.Cmd) {
	c.width = msg.Width
	return c, nil
}

func (c DialogModel) View() string {
	return c.getContent()
}

func (c DialogModel) getContent() string {
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

	// Render the question as markdown using glamour
	renderer, _ := glamour.NewTermRenderer(glamour.WithWordWrap(c.width-1), glamour.WithStandardStyle("dark"))
	rendered, _ := renderer.Render(c.question)

	s := strings.Builder{}
	s.WriteString(rendered)
	s.WriteString("\n\n")
	s.WriteString("\t1. Yes\n")
	s.WriteString("\t2. No\n")

	return style.Render(s.String())
}

func (c DialogModel) Width() int {
	return c.width
}

func (c DialogModel) handleConfirmationRequest(msg ConfirmationRequest) (DialogModel, tea.Cmd) {
	c.question = msg.Question
	c.visible = true
	return c, nil
}

func (c DialogModel) Visible() bool {
	return c.visible
}

func (c DialogModel) GetHeight() int {
	if !c.visible {
		return 0
	}
	return lipgloss.Height(c.getContent())
}
