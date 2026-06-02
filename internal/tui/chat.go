package tui

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/rs/zerolog/log"
)

type Model struct {
	messages      []ChatMessage
	agentChunks   string
	textarea      textarea.Model
	viewport      viewport.Model
	confirmDialog DialogModel
	width         int
	height        int
	userChan      chan any
	blenoffset    int
	spinner       spinner.Model
	agentActive   bool

	currentConfirmationRequest ConfirmationRequest
}

func InitialModel(userChan chan any) tea.Model {
	ta := textarea.New()
	ta.Placeholder = "Type a message and press Enter..."
	ta.SetVirtualCursor(false)
	ta.Focus()

	ta.Prompt = "┃ "
	ta.CharLimit = 4096

	ta.SetWidth(30)
	ta.SetHeight(3)

	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(true)

	vp := viewport.New(viewport.WithWidth(30), viewport.WithHeight(5))
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)

	s := spinner.New()
	s.Spinner = spinner.Points

	p := Model{
		textarea:      ta,
		viewport:      vp,
		messages:      []ChatMessage{},
		userChan:      userChan,
		confirmDialog: InitialDialogModel(),
		blenoffset:    0,
		spinner:       s,
	}

	return p
}

func (p Model) Init() tea.Cmd {
	p.spinner.Spinner = spinner.Dot
	return tea.Batch(tickEvery(), p.spinner.Tick)
}

func (p Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var mCmd, dCmd tea.Cmd
	switch msg := msg.(type) {
	case ConfirmationRequest:
		p, mCmd = p.handleConfirmationRequest(msg)

	case MessageChunk:
		p, mCmd = p.handleAgentChunk(msg)

	case MessageFull:
		p, mCmd = p.handleAgentInput(msg)

	case tea.WindowSizeMsg:
		p, mCmd = p.handleWindowResize(msg)

	case tea.PasteMsg:
		p, mCmd = p.handlePaste(msg)

	case tea.KeyPressMsg:
		p, mCmd = p.handleUserInput(msg)

	case DecisionMessage:
		p, mCmd = p.handleUserInputConfirmation(msg)

	case TickMsg:
		p, mCmd = p.handleTick(msg)

	case spinner.TickMsg:
		p, mCmd = p.handleSpinnerTick(msg)

	case AgentStart:
		p, mCmd = p.handleAgentStart(msg)

	case AgentStop:
		p, mCmd = p.handleAgentStop(msg)
	}

	p.confirmDialog, dCmd = p.confirmDialog.Update(msg)

	if _, ok := msg.(ConfirmationRequest); ok {
		p = p.resizeComponents()
	}

	return p, tea.Batch(mCmd, dCmd)
}

func (p Model) handleAgentStart(_ AgentStart) (Model, tea.Cmd) {
	p.agentActive = true
	return p.refreshMessages()
}

func (p Model) handleAgentStop(_ AgentStop) (Model, tea.Cmd) {
	p.agentActive = false
	return p.refreshMessages()
}

func (p Model) handleTick(_ tea.Msg) (Model, tea.Cmd) {
	log.Logger.Debug().Msgf("received tick message")
	return p, nil
}

func (p Model) handleSpinnerTick(msg tea.Msg) (Model, tea.Cmd) {
	var cmd1, cmd2 tea.Cmd
	p.spinner, cmd1 = p.spinner.Update(msg)

	if p.agentActive {
		p, cmd2 = p.refreshMessages()
	}
	return p, tea.Batch(cmd1, cmd2)
}

func (p Model) handleConfirmationRequest(msg ConfirmationRequest) (Model, tea.Cmd) {
	log.Logger.Info().Msgf("received confirmation request: %#v", msg)
	p.currentConfirmationRequest = msg
	return p, nil
}

func (p Model) handleAgentChunk(msg MessageChunk) (Model, tea.Cmd) {
	log.Logger.Debug().Msg("handleAgentChunk()")
	p.agentChunks += string(msg.Content)
	return p.refreshMessages()
}

func (p Model) handleAgentInput(msg MessageFull) (Model, tea.Cmd) {
	log.Logger.Debug().Msg("handleAgentInput()")
	p.agentChunks = ""
	p.messages = append(p.messages, NewChatMessage("Cosmo", msg.Content))
	return p.refreshMessages()
}

func (p Model) handlePaste(msg tea.PasteMsg) (Model, tea.Cmd) {
	p.textarea.InsertString(msg.Content)
	return p, nil
}

func (p Model) handleWindowResize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	p.width = msg.Width
	p.height = msg.Height

	return p.resizeComponents(), nil
}

func (p Model) resizeComponents() Model {
	p.viewport.SetWidth(p.width)
	p.viewport.SetHeight(p.height - p.textarea.Height() - p.confirmDialog.GetHeight())
	p.textarea.SetWidth(p.width)
	return p
}

func (p Model) handleUserInput(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	if p.confirmDialog.Visible() {
		// skip updates if we're sending key messages to the dialog
		return p, nil
	}

	var cmd tea.Cmd
	switch msg.String() {
	case "esc", "ctrl+c":
		return p, tea.Quit
	case "shift+up":
		log.Logger.Debug().Msg("scroll up!")
		p.viewport.ScrollUp(5)
	case "shift+down":
		log.Logger.Debug().Msg("scroll down!")
		p.viewport.ScrollDown(5)
	case "enter":
		text := p.textarea.Value()
		p.messages = append(p.messages, NewChatMessage("You", text))
		p.textarea.Reset()
		p.userChan <- text

		return p.refreshMessages()
	default:
		// All other keypresses — pass through to the textarea
		p.textarea, cmd = p.textarea.Update(msg)
	}

	return p, cmd
}

func (p Model) handleUserInputConfirmation(msg DecisionMessage) (Model, tea.Cmd) {
	var dCmd tea.Cmd
	p.confirmDialog, dCmd = p.confirmDialog.handleDecision(msg)

	p = p.resizeComponents()

	ch := p.currentConfirmationRequest.ResultChan
	cmd := func() tea.Msg {
		ch <- msg.Approved
		return nil
	}

	return p, tea.Batch(cmd, dCmd)
}

func (p Model) refreshMessages() (Model, tea.Cmd) {
	wasAtBottom := p.viewport.AtBottom()
	var contentMessages []ChatMessage
	if p.agentActive {
		contentMessages = append(p.messages, NewChatMessage("Cosmo", p.agentChunks+" "+p.spinner.View()))
	} else {
		contentMessages = p.messages
	}

	cosmoColor := lipgloss.Color("#9900ff")
	youColor := lipgloss.Color("#FF69B4")

	cosmoBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(cosmoColor)

	youBorderStyle := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).BorderForeground(youColor)

	//senderStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	renderer, _ := glamour.NewTermRenderer(glamour.WithWordWrap(p.width-1), glamour.WithStandardStyle("dracula"))

	var content []string
	for _, msg := range contentMessages {
		rendered, _ := renderer.Render(msg.Content)

		if strings.Contains(msg.Sender, "Cosmo") {
			rendered = cosmoBorderStyle.Render(rendered)
		} else {
			rendered = youBorderStyle.Render(rendered)
		}

		cs := []string{msg.Sender, rendered}
		content = append(content, cs...)
	}

	p.viewport.SetContentLines(content)
	if wasAtBottom {
		p.viewport.GotoBottom()
	}

	return p, nil
}

func (p Model) View() tea.View {
	viewportView := p.viewport.View()

	var allViews string
	if p.confirmDialog.Visible() {
		allViews = viewportView + "\n" + p.confirmDialog.View() + "\n" + p.textarea.View()
	} else {
		allViews = viewportView + "\n" + p.textarea.View()
	}

	v := tea.NewView(allViews)
	c := p.textarea.Cursor()
	if c != nil {
		c.Y += lipgloss.Height(viewportView)
	}
	v.Cursor = c
	v.AltScreen = false
	return v
}
