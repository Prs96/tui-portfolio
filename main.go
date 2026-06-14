package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── SSH server config ─────────────────────────────────────────────────────────

const (
	sshHost    = "0.0.0.0"
	sshPort    = "443"
	hostKeyDir = ".ssh/portfolio_ed25519"
)

type sessionState int

const (
	stateMenu sessionState = iota
	stateAboutMe
	stateProjects
	statePublications
	stateSkills
	stateContact
)

// tickMsg drives the animation clock
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(420*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	state          sessionState
	choices        []string
	cursor         int
	terminalWidth  int
	terminalHeight int
	frame          int // animation frame counter
	quoteIdx       int // current quote index for welcome page
}

func initialModel() model {
	return model{
		state:    stateMenu,
		choices:  []string{"About Me", "Projects", "Publications", "Skills", "Contact"},
		cursor:   0,
		quoteIdx: 0,
	}
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.frame++
		if m.frame%12 == 0 {
			m.quoteIdx = (m.quoteIdx + 1) % 4
		}
		return m, tick()

	case tea.WindowSizeMsg:
		m.terminalWidth = msg.Width
		m.terminalHeight = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc":
			m.state = stateMenu
			m.cursor = 0
		// Added explicit matches for standard ANSI escape sequences common over raw SSH pipes
		case "up", "k", "\x1b[A":
			if m.cursor > 0 {
				m.cursor--
				m.state = sessionState(m.cursor + 1)
			}
		case "down", "j", "\x1b[B":
			if m.cursor < len(m.choices)-1 {
				m.cursor++
				m.state = sessionState(m.cursor + 1)
			}
		case "enter", " ":
			m.state = sessionState(m.cursor + 1)
		}
	}
	return m, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func rule(width int, col lipgloss.Color) string {
	return lipgloss.NewStyle().Foreground(col).Render(strings.Repeat("─", width))
}

func padR(s string, w int) string {
	n := lipgloss.Width(s)
	if n >= w {
		return s
	}
	return s + strings.Repeat(" ", w-n)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── ASCII art ─────────────────────────────────────────────────────────────────

func coffeeFrames() [2][]string {
	return [2][]string{
		{
			"  )  )  ) ",
			" (  (  (  ",
			"  )  )  ) ",
			" .---------. ",
			" |  ( (    | ",
			" |  ) )  ) |_|",
			" |  ( (  (  |_)",
			" `-----------'",
			"   `---------'",
			"",
		},
		{
			" )  )  )  ",
			"  (  (  ( ",
			" )  )  )  ",
			" .---------. ",
			" |  ) )    | ",
			" |  ( (  ( |_|",
			" |  ) )  ) |_)",
			" `-----------'",
			"   `---------'",
			"",
		},
	}
}

func coffeeArt(frame int) []string {
	steams := [2][3]string{
		{`  ~   ~  ~ `, ` ~   ~   ~ `, `  ~  ~  ~  `},
		{` ~   ~   ~ `, `  ~   ~  ~ `, ` ~    ~  ~ `},
	}
	s := steams[frame%2]

	return []string{
		s[0],
		s[1],
		s[2],
		`  .-----------.  `,
		`  |  ~ ~ ~ ~  |  `,
		`  |   ( ( (   |\ `,
		`  |    ) ) )  | )`,
		`  |   ( ( (   |/ `,
		`  '-------------'`,
		`    '-----------' `,
	}
}

func pranavASCII() []string {
	return []string{
		` ██████╗ ██████╗  █████╗ ███╗   ██╗ █████╗ ██╗   ██╗`,
		` ██╔══██╗██╔══██╗██╔══██╗████╗  ██║██╔══██╗██║   ██║`,
		` ██████╔╝██████╔╝███████║██╔██╗ ██║███████║██║   ██║`,
		` ██╔═══╝ ██╔══██╗██╔══██║██║╚██╗██║██╔══██║╚██╗ ██╔╝`,
		` ██║     ██║  ██║██║  ██║██║ ╚████║██║  ██║ ╚████╔╝ `,
		` ╚═╝     ╚═╝  ╚═╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝  ╚═══╝ `,
	}
}

// ── View ──────────────────────────────────────────────────────────────────────

func (m model) View() string {
	if m.terminalWidth < 90 || m.terminalHeight < 18 {
		return "\n  Terminal too small — please resize to at least 90 × 18."
	}

	// ── Palette ──────────────────────────────────────────────────────────────
	emerald := lipgloss.Color("#10B981")
	violet := lipgloss.Color("#A78BFA")
	teal := lipgloss.Color("#2DD4BF")
	silver := lipgloss.Color("#CBD5E1")
	muted := lipgloss.Color("#475569")
	dim := lipgloss.Color("#1E293B")
	white := lipgloss.Color("#F1F5F9")
	amber := lipgloss.Color("#FBBF24")

	// ── Dimensions ───────────────────────────────────────────────────────────
	sidebarOuter := 28
	sidebarInner := sidebarOuter - 4
	boxHeight := m.terminalHeight - 5
	contentOuter := m.terminalWidth - sidebarOuter - 1 - 4
	if contentOuter < 56 {
		contentOuter = 56
	}
	innerW := contentOuter - 10

	// ── Base styles ──────────────────────────────────────────────────────────
	titleSt := lipgloss.NewStyle().Bold(true).Foreground(emerald)
	headSt := lipgloss.NewStyle().Bold(true).Foreground(white)
	bodySt := lipgloss.NewStyle().Foreground(silver)
	hintSt := lipgloss.NewStyle().Foreground(muted).Italic(true)
	tagSt := lipgloss.NewStyle().Foreground(teal)
	dateSt := lipgloss.NewStyle().Foreground(teal).Italic(true)
	lblSt := lipgloss.NewStyle().Bold(true).Foreground(silver)
	selText := lipgloss.NewStyle().Foreground(violet).Bold(true)
	selMark := lipgloss.NewStyle().Foreground(violet).Bold(true)
	steamSt := lipgloss.NewStyle().Foreground(amber)
	cupSt := lipgloss.NewStyle().Foreground(lipgloss.Color("#92400E"))
	nameSt := lipgloss.NewStyle().Bold(true).Foreground(emerald)

	// ── Sidebar ───────────────────────────────────────────────────────────────
	badgeInner := sidebarInner - 2
	nameLine := lipgloss.NewStyle().Bold(true).Foreground(emerald).
		Width(badgeInner).Align(lipgloss.Center).Render("PRANAV  S")
	subLine := lipgloss.NewStyle().Foreground(muted).
		Width(badgeInner).Align(lipgloss.Center).Render("SOFTWARE DEVELOPER")

	topBar := lipgloss.NewStyle().Foreground(emerald).Render("╔" + strings.Repeat("═", badgeInner) + "╗")
	midName := lipgloss.NewStyle().Foreground(emerald).Render("║") + nameLine + lipgloss.NewStyle().Foreground(emerald).Render("║")
	midSep := lipgloss.NewStyle().Foreground(muted).Render("╟" + strings.Repeat("─", badgeInner) + "╢")
	midSub := lipgloss.NewStyle().Foreground(muted).Render("║") + subLine + lipgloss.NewStyle().Foreground(muted).Render("║")
	botBar := lipgloss.NewStyle().Foreground(muted).Render("╚" + strings.Repeat("═", badgeInner) + "╝")
	badge := topBar + "\n" + midName + "\n" + midSep + "\n" + midSub + "\n" + botBar

	sidebar := badge + "\n\n"
	sidebar += lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("·", sidebarInner)) + "\n\n"

	icons := []string{"○", "○", "○", "○", "○"}
	for i, choice := range m.choices {
		if m.cursor == i {
			icons[i] = "◉"
			sidebar += fmt.Sprintf("  %s  %s\n",
				selMark.Render("▶ "+icons[i]),
				selText.Render(choice))
		} else {
			sidebar += fmt.Sprintf("  %s  %s\n",
				lipgloss.NewStyle().Foreground(muted).Render("  "+icons[i]),
				bodySt.Render(choice))
		}
	}

	sidebar += "\n" + lipgloss.NewStyle().Foreground(dim).Render(strings.Repeat("·", sidebarInner)) + "\n\n"
	sidebar += hintSt.Render("  ↑ k   up") + "\n"
	sidebar += hintSt.Render("  ↓ j   down") + "\n"
	sidebar += hintSt.Render("  q     quit")

	sidebarBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(muted).
		Padding(1, 2).
		Width(sidebarOuter).
		Height(boxHeight).
		Render(sidebar)

	// ── Content helpers ───────────────────────────────────────────────────────
	sectionTitle := func(text string) string {
		bar := lipgloss.NewStyle().Foreground(emerald).Render("▌")
		return bar + titleSt.Render(" "+text) + "\n" + rule(innerW, dim)
	}
	cardTop := func(w int) string {
		return lipgloss.NewStyle().Foreground(muted).Render("┌" + strings.Repeat("─", w-1))
	}

	var content string

	switch m.state {

	case stateMenu:
		cup := coffeeArt(m.frame)
		name := pranavASCII()

		cupRendered := make([]string, len(cup))
		for i, row := range cup {
			if i < 3 {
				cupRendered[i] = steamSt.Render(row)
			} else {
				cupRendered[i] = cupSt.Render(row)
			}
		}

		nameRendered := make([]string, len(name))
		for i, row := range name {
			nameRendered[i] = nameSt.Render(row)
		}

		cupBlock := strings.Join(cupRendered, "\n")
		nameBlock := strings.Join(nameRendered, "\n")

		centreStyle := lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center)

		content += "\n"
		content += centreStyle.Render(nameBlock) + "\n\n"
		content += centreStyle.Render(cupBlock) + "\n\n"
		content += rule(innerW, dim) + "\n"

		quotes := []string{
			"\"The best way to predict the future is to invent it.\"",
			"\"Simplicity is the ultimate sophistication.\"",
			"\"First, solve the problem. Then, write the code.\"",
			"\"In the middle of difficulty lies opportunity.\"",
		}
		quoteSt := lipgloss.NewStyle().Foreground(amber).Italic(true).Width(innerW - 4).Align(lipgloss.Center)
		content += "\n" + centreStyle.Render(quoteSt.Render(quotes[m.quoteIdx]))

	case stateAboutMe:
		content += bodySt.Render("Hi, I'm ") + headSt.Render("Pranav S") +
			bodySt.Render(", a Computer Science and Engineering student at the ") +
			tagSt.Render("College of Engineering Trivandrum") +
			bodySt.Render(".\n\n")
		content += bodySt.Render("I work with ") + tagSt.Render("Machine Learning, AI, and Software Development") +
			bodySt.Render(", with interests in computer vision, automation, and intelligent systems. I enjoy building projects that combine research and engineering to solve practical problems, while continuously exploring new technologies and development tools.") + "\n\n"

		content += rule(innerW, dim) + "\n\n"
		content += sectionTitle("EXPERIENCE") + "\n\n"

		content += cardTop(innerW) + "\n"
		content += headSt.Render(" Machine Learning Developer Intern") + "\n"
		content += fmt.Sprintf(" %s  %s\n",
			tagSt.Render("DICEMED"),
			dateSt.Render("Mar 2025 – Dec 2025  ·  Remote"))
		content += "\n"
		for _, b := range []string{
			"Built end-to-end ML pipelines for dental diagnostics and CBCT segmentation.",
			"Two-stage pericoronitis detection (YOLOv8 + ResNet-50): 92% precision,\n   92.5% mAP, 84% radiologist alignment via Grad-CAM.",
			"Hybrid nnU-Net + interactive 3D U-Net for multi-structure CBCT segmentation —\n   0.74 pseudo-Dice score across 77 classes.",
		} {
			content += bodySt.Render("  · "+b) + "\n"
		}
		content += "\n"

		content += cardTop(innerW) + "\n"
		content += headSt.Render(" Technical Lead") + "\n"
		content += fmt.Sprintf(" %s  %s\n",
			tagSt.Render("GLITCH CET"),
			dateSt.Render("Apr 2025 – Present"))
		content += "\n"
		for _, b := range []string{
			"Spearheaded agile development lifecycles for student game design projects.",
			"Organised campus technical workshops on game development engineering.",
		} {
			content += bodySt.Render("  · "+b) + "\n"
		}

	case stateProjects:
		content += sectionTitle("PROJECTS") + "\n\n"

		type proj struct {
			name, stack, url string
			bullets          []string
		}
		for _, p := range []proj{
			{
				"LLM Determinism Engine",
				"Python · PyTorch · CAA / GCAV",
				"https://github.com/huephen/hcx-001.git",
				[]string{
					"3-stage pipeline (Normalise → Verify → Steer) enforcing semantic\n   consistency for legal/medical LLM deployments.",
					"Activation steering on Qwen — 0.985 semantic similarity score.",
				},
			},
			{
				"Rental Management Platform",
				"TypeScript · Next.js · Tailwind CSS",
				"https://github.com/Prs96/rental-management.git",
				[]string{
					"High-performance SSR frontend with optimised core web vitals.",
					"Integrated secure authentication and real-time data fetching by consuming RESTful APIs to manage property listings and user bookings.",
				},
			},
			{
				"Interactive Segmentation",
				"Python · PyTorch · nnU-Net",
				"https://github.com/Prs96/Interactive_Segmentation.git",
				[]string{
					"Built end-to-end ML pipelines for dental diagnostics and CBCT segmentation.",
					"Designed a hybrid nnU-Net + interactive 3D U-Net framework for CBCT multi-structure segmentation, achieving 0.74±0.03 pseudo-Dice (77 classes) and improving IAC Dice to 0.86.",
				},
			},
			{
				"Stack It",
				"TypeScript · React",
				"https://github.com/Prs96/StackIt.git",
				[]string{
					"StackIt is a minimal question-and-answer platform that supports collaborative learning and structured knowledge sharing. It's designed to be simple, user-friendly, and focused on the core experience of asking and answering questions within a community.",
				},
			},
			{
				"Project Five",
				"TBD",
				"",
				[]string{"Placeholder — awaiting content."},
			},
		} {
			content += cardTop(innerW) + "\n"
			content += headSt.Render(" "+p.name) + "\n"
			if p.url != "" {
				link := fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", p.url, p.url)
				content += " " + tagSt.Render(p.stack) + "  " + hintSt.Render(link) + "\n\n"
			} else {
				content += " " + tagSt.Render(p.stack) + "\n\n"
			}
			for _, b := range p.bullets {
				content += bodySt.Render("  · "+b) + "\n"
			}
			content += "\n"
		}

	case statePublications:
		content += sectionTitle("PUBLICATIONS") + "\n\n"

		content += rule(innerW, dim) + "\n"
		for _, p := range []struct{ title, venue, url string }{
			{
				"Automated Multi-Structure Segmentation in Dental CBCT\n   using nnU-Net with Interactive IAC Refinement",
				"ToothFairy3 Challenge · AIHC 2025",
				"",
			},
			{
				"Explainable Two-Stage Deep Learning Framework for\n   Pericoronitis Assessment using YOLOv8 and ResNet-50",
				"arXiv Preprint · 2025",
				"https://arxiv.org/pdf/2601.08401",
			},
		} {
			content += bodySt.Render("  · "+p.title) + "\n"
			venue := hintSt.Render("    " + p.venue)
			if p.url != "" {
				venue = hintSt.Render("    " + fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", p.url, p.venue))
			}
			content += venue + "\n\n"
		}

	case stateSkills:
		content += sectionTitle("SKILLS & HONOURS") + "\n\n"

		for _, row := range []struct{ label, value string }{
			{"Languages ", "Python · Go · C · Rust · Java · JS · TypeScript · HTML/CSS"},
			{"Frameworks", "React · Next.js · Django · Flask · PyTorch · Docker · Linux"},
			{"Domains   ", "Machine Learning · Computer Vision · Web · Systems"},
		} {
			content += fmt.Sprintf("  %s  %s\n", lblSt.Render(row.label), bodySt.Render(row.value))
		}

		content += "\n" + rule(innerW, dim) + "\n"
		content += lipgloss.NewStyle().Bold(true).Foreground(violet).Render("▌") +
			lblSt.Render(" HONOURS") + "\n\n"
		for _, h := range []struct{ badge, text string }{
			{"🏆", "Finalist — Odoo National Level Hackathon"},
			{"🥇", "Top 10 Rank — UST D3CODE Hackathon"},
			{"📜", "Supervised Machine Learning · Stanford Online"},
			{"📜", "Advanced Learning Algorithms · Stanford Online"},
		} {
			content += fmt.Sprintf("  %s  %s\n", h.badge, bodySt.Render(h.text))
		}

	case stateContact:
		content += sectionTitle("CONTACT") + "\n\n"
		content += bodySt.Render("Open to opportunities — reach out through any channel below.\n\n")
		content += rule(innerW, dim) + "\n\n"

		for _, c := range []struct {
			icon, label, value, url string
		}{
			{"✉", "Email   ", "pranavsudheesh34@gmail.com", ""},
			{"⌥", "GitHub  ", "github.com/pranav", "https://github.com/pranav"},
			{"in", "LinkedIn", "linkedin.com/in/pranav", "https://linkedin.com/in/pranav"},
			{"☎", "Phone   ", "+91 89217 11927", ""},
		} {
			val := bodySt.Render(c.value)
			if c.url != "" {
				val = fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", c.url, bodySt.Render(c.value))
			}
			content += fmt.Sprintf("  %s  %s  %s\n",
				lipgloss.NewStyle().Foreground(teal).Render(c.icon),
				lblSt.Render(padR(c.label, 9)),
				val)
		}

		content += "\n\n" + rule(innerW, dim) + "\n"
		content += hintSt.Render("  Node status: online · listening for connections")
	}

	contentBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(emerald).
		Padding(1, 4).
		Width(contentOuter).
		Height(boxHeight).
		Render(content)

	// ── Status bar ────────────────────────────────────────────────────────────
	sections := []string{"Welcome", "About Me", "Projects", "Publications", "Skills", "Contact"}
	leftPart := hintSt.Render("  pranav-s.dev  ·  q quit  ·  ↑↓ navigate")
	rightPart := hintSt.Render(sections[m.state] + "  ")
	barW := m.terminalWidth - 4
	spacer := strings.Repeat(" ", maxInt(0, barW-lipgloss.Width(leftPart)-lipgloss.Width(rightPart)))
	statusBar := lipgloss.NewStyle().Foreground(muted).Render(leftPart + spacer + rightPart)

	// ── Assemble ──────────────────────────────────────────────────────────────
	dashboard := lipgloss.JoinHorizontal(lipgloss.Top, sidebarBox, contentBox)
	return lipgloss.Place(
		m.terminalWidth, m.terminalHeight,
		lipgloss.Center, lipgloss.Center,
		dashboard+"\n"+statusBar,
	)
}

// ── SSH Bridge Entrypoint ─────────────────────────────────────────────────────

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, ok := s.Pty()
	m := initialModel()
	if ok {
		m.terminalWidth = pty.Window.Width
		m.terminalHeight = pty.Window.Height
	} else {
		m.terminalWidth = 80
		m.terminalHeight = 24
	}
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

func main() {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf("%s:%s", sshHost, sshPort)),
		wish.WithHostKeyPath(hostKeyDir),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			logging.Middleware(),
		),
	)
	if err != nil {
		fmt.Printf("could not start SSH server: %v\n", err)
		os.Exit(1)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("SSH portfolio listening on %s:%s\n", sshHost, sshPort)

	go func() {
		if err := s.ListenAndServe(); err != nil && err != ssh.ErrServerClosed {
			fmt.Printf("server error: %v\n", err)
			done <- syscall.SIGTERM
		}
	}()

	<-done
	fmt.Println("\nShutting down server gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.Shutdown(ctx); err != nil {
		fmt.Printf("shutdown error: %v\n", err)
	}
}
