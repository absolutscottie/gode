package tui

import (
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"

	"github.com/rs/zerolog/log"
)

type MessageChunk struct {
	Content string
}

type MessageFull struct {
	Content string
}

type ConfirmationRequest struct {
	ResultChan chan bool
	Question   string
}

type ChatMessage struct {
	Sender  string
	Content string
}

func NewChatMessage(sender, content string) ChatMessage {
	return ChatMessage{
		Sender:  sender,
		Content: content,
	}
}

type Model struct {
	messages      []ChatMessage
	agentChunks   string
	textarea      textarea.Model
	viewport      viewport.Model
	confirmDialog *ConfirmationDialog
	width         int
	height        int
	userChan      chan any

	currentConfirmationRequest ConfirmationRequest
}

func InitialModel(userChan chan any) Model {
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

	p := Model{
		textarea:      ta,
		viewport:      vp,
		messages:      []ChatMessage{},
		userChan:      userChan,
		confirmDialog: NewConfirmationDialog(),
	}

	return p
}

func (p Model) Init() tea.Cmd {
	return nil
}

func (p Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ConfirmationRequest:
		return p.handleConfirmationRequest(msg)

	case MessageChunk:
		return p.handleAgentChunk(msg)

	case MessageFull:
		return p.handleAgentInput(msg)

	case tea.WindowSizeMsg:
		return p.handleWindowResize(msg)

	case tea.PasteMsg:
		return p.handlePaste(msg)

	case tea.KeyPressMsg:
		return p.handleUserInput(msg)

	}

	return p, nil
}

func (p Model) handleConfirmationRequest(msg ConfirmationRequest) (tea.Model, tea.Cmd) {
	log.Logger.Info().Msgf("received confirmation request: %#v", msg)
	p.confirmDialog.SetVisible(true)
	p.confirmDialog.SetQuestion(msg.Question)
	p.currentConfirmationRequest = msg
	return p, nil
}

func (p Model) handleAgentChunk(msg MessageChunk) (tea.Model, tea.Cmd) {
	log.Logger.Debug().Msg("handleAgentChunk()")
	p.agentChunks += string(msg.Content)
	return p.refreshMessages()
}

func (p Model) handleAgentInput(msg MessageFull) (tea.Model, tea.Cmd) {
	log.Logger.Debug().Msg("handleAgentInput()")
	p.agentChunks = ""
	p.messages = append(p.messages, NewChatMessage("Cosmo", msg.Content))
	return p.refreshMessages()
}

func (p Model) handlePaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	p.textarea.InsertString(msg.Content)
	return p, nil
}

func (p Model) handleWindowResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	p.width = msg.Width
	p.height = msg.Height

	return p.resizeComponents(), nil
}

func (p Model) resizeComponents() tea.Model {
	p.confirmDialog.SetWidth(p.width)
	p.viewport.SetWidth(p.width)
	p.viewport.SetHeight(p.height - p.textarea.Height() - p.confirmDialog.GetHeight())
	p.textarea.SetWidth(p.width)
	return p
}

func (p Model) handleUserInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if p.confirmDialog.Visible() {
		return p.handleUserInputConfirmation(msg)
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

func (p Model) handleUserInputConfirmation(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1": //Yes
		p.currentConfirmationRequest.ResultChan <- true
	case "2": //No
		p.currentConfirmationRequest.ResultChan <- false
	}

	p.confirmDialog.SetVisible(false)
	return p, nil
}

func (p Model) refreshMessages() (tea.Model, tea.Cmd) {
	wasAtBottom := p.viewport.AtBottom()
	var contentMessages []ChatMessage
	if p.agentChunks != "" {
		contentMessages = append(p.messages, NewChatMessage("Cosmo", p.agentChunks))
	} else {
		contentMessages = p.messages
	}

	senderStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	renderer, _ := glamour.NewTermRenderer(glamour.WithWordWrap(p.width), glamour.WithStandardStyle("dark"))

	var content []string
	for _, msg := range contentMessages {
		rendered, _ := renderer.Render(msg.Content)
		cs := []string{senderStyle.Render(msg.Sender), rendered}
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
