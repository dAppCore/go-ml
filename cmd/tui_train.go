//go:build darwin && arm64 && !nomlx && cliv1

package cmd

import (
	"math"
	"time"

	"dappco.re/go"
	"dappco.re/go/cli/pkg/cli"
	tea "github.com/charmbracelet/bubbletea"
)

// Compile-time checks.
var (
	_ cli.FrameModel = (*trainStatusModel)(nil)
	_ cli.FrameModel = (*trainContentModel)(nil)
	_ cli.FrameModel = (*trainHintsModel)(nil)
)

// --- Messages ---

// TrainTickMsg carries a snapshot of training state from the loop goroutine.
type TrainTickMsg struct {
	Iter       int
	TotalIters int
	Loss       float64
	ValLoss    float64
	LR         float64
	TokensPerS float64
	PeakMemGB  float64
	Tokens     int64
	Phase      string
	RunID      string
	Done       bool
	Err        error
}

// --- Header: Status bar ---

type trainStatusModel struct {
	tick TrainTickMsg
}

func newTrainStatusModel() *trainStatusModel {
	return &trainStatusModel{}
}

func (m *trainStatusModel) Init() tea.Cmd { return nil }

func (m *trainStatusModel) Update(msg tea.Msg) (cli.FrameModel, tea.Cmd) {
	if t, ok := msg.(TrainTickMsg); ok {
		m.tick = t
	}
	return m, nil
}

func (m *trainStatusModel) View(width, _ int) string {
	t := m.tick
	if t.TotalIters == 0 {
		return " LEM training | loading..."
	}

	pct := float64(t.Iter) / float64(t.TotalIters) * 100
	status := "training"
	if t.Done {
		status = "complete"
	}
	if t.Err != nil {
		status = "error"
	}

	line := core.Sprintf(" LEM %s | %s | iter %d/%d (%.0f%%) | loss %.4f | %.0f tok/s | %.1fGB",
		t.Phase, status, t.Iter, t.TotalIters, pct, t.Loss, t.TokensPerS, t.PeakMemGB)

	if width > 0 && len(line) > width {
		line = line[:width]
	}
	return line
}

// --- Content: Loss chart + metrics ---

type trainContentModel struct {
	tick     TrainTickMsg
	lossHist []float64
	valHist  []float64
	valIters []int
	width    int
	height   int
}

func newTrainContentModel() *trainContentModel {
	return &trainContentModel{}
}

func (m *trainContentModel) Init() tea.Cmd { return nil }

func (m *trainContentModel) Update(msg tea.Msg) (cli.FrameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case TrainTickMsg:
		m.tick = msg
		if msg.Loss > 0 {
			m.lossHist = append(m.lossHist, msg.Loss)
		}
		if msg.ValLoss > 0 {
			m.valHist = append(m.valHist, msg.ValLoss)
			m.valIters = append(m.valIters, msg.Iter)
		}
	}
	return m, nil
}

func (m *trainContentModel) View(width, height int) string {
	m.width = width
	m.height = height

	if len(m.lossHist) == 0 {
		return " waiting for first training step..."
	}

	b := core.NewBuilder()

	// --- Progress bar ---
	t := m.tick
	barWidth := width - 20
	if barWidth < 10 {
		barWidth = 10
	}
	pct := float64(t.Iter) / float64(t.TotalIters)
	filled := int(pct * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}
	bar := repeatString("█", filled) + repeatString("░", barWidth-filled)
	_, _ = b.WriteString(core.Sprintf(" [%s] %3.0f%%\n\n", bar, pct*100))

	// --- Loss chart ---
	chartHeight := height - 10
	if chartHeight < 5 {
		chartHeight = 5
	}
	chartWidth := width - 12
	if chartWidth < 20 {
		chartWidth = 20
	}

	_, _ = b.WriteString(renderLossChart(m.lossHist, m.valHist, chartWidth, chartHeight))
	if err := b.WriteByte('\n'); err != nil {
		return b.String()
	}

	// --- Metrics table ---
	_, _ = b.WriteString(core.Sprintf(" iteration:  %d / %d\n", t.Iter, t.TotalIters))
	_, _ = b.WriteString(core.Sprintf(" train loss: %.4f  (ppl %.2f)\n", t.Loss, math.Exp(math.Min(t.Loss, 20))))
	if t.ValLoss > 0 {
		_, _ = b.WriteString(core.Sprintf(" val loss:   %.4f  (ppl %.2f)\n", t.ValLoss, math.Exp(math.Min(t.ValLoss, 20))))
	}
	_, _ = b.WriteString(core.Sprintf(" lr:         %.2e\n", t.LR))
	_, _ = b.WriteString(core.Sprintf(" throughput: %.0f tok/s\n", t.TokensPerS))
	_, _ = b.WriteString(core.Sprintf(" peak mem:   %.1f GB\n", t.PeakMemGB))
	_, _ = b.WriteString(core.Sprintf(" tokens:     %d\n", t.Tokens))

	if t.Done {
		_, _ = b.WriteString("\n training complete!")
	}
	if t.Err != nil {
		_, _ = b.WriteString(core.Sprintf("\n error: %v", t.Err))
	}

	return b.String()
}

// renderLossChart draws an ASCII sparkline chart of training (and optionally validation) loss.
func renderLossChart(train, val []float64, width, height int) string {
	if len(train) == 0 || height < 2 || width < 5 {
		return ""
	}

	// Downsample train loss to fit width
	points := downsample(train, width)

	// Find range
	minV, maxV := points[0], points[0]
	for _, v := range points {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}
	for _, v := range val {
		if v < minV {
			minV = v
		}
		if v > maxV {
			maxV = v
		}
	}

	// Add margin
	span := maxV - minV
	if span < 0.01 {
		span = 0.01
	}

	// Build character grid
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

	b := core.NewBuilder()

	// Y-axis labels + chart
	_, _ = b.WriteString(core.Sprintf(" %6.3f ┐\n", maxV))

	for row := height - 1; row >= 0; row-- {
		rowMin := minV + span*float64(row)/float64(height)
		rowMax := minV + span*float64(row+1)/float64(height)

		if row == height/2 {
			_, _ = b.WriteString(core.Sprintf(" %6.3f │", (rowMin+rowMax)/2))
		} else {
			_, _ = b.WriteString("        │")
		}

		for col := range len(points) {
			v := points[col]
			if v >= rowMin && v < rowMax {
				// Fractional position within this row
				frac := (v - rowMin) / (rowMax - rowMin)
				idx := int(frac * float64(len(blocks)-1))
				_, _ = b.WriteRune(blocks[idx])
			} else if v >= rowMax {
				_, _ = b.WriteRune('█')
			} else {
				_, _ = b.WriteRune(' ')
			}
		}
		if err := b.WriteByte('\n'); err != nil {
			return b.String()
		}
	}

	_, _ = b.WriteString(core.Sprintf(" %6.3f └%s\n", minV, repeatString("─", len(points))))

	return b.String()
}

// downsample reduces a slice to at most n points by averaging buckets.
func downsample(data []float64, n int) []float64 {
	if len(data) <= n {
		return data
	}
	out := make([]float64, n)
	bucket := float64(len(data)) / float64(n)
	for i := range n {
		start := int(float64(i) * bucket)
		end := int(float64(i+1) * bucket)
		if end > len(data) {
			end = len(data)
		}
		var sum float64
		for j := start; j < end; j++ {
			sum += data[j]
		}
		out[i] = sum / float64(end-start)
	}
	return out
}

// --- Footer: Key hints ---

type trainHintsModel struct {
	done bool
}

func newTrainHintsModel() *trainHintsModel {
	return &trainHintsModel{}
}

func (m *trainHintsModel) Init() tea.Cmd { return nil }

func (m *trainHintsModel) Update(msg tea.Msg) (cli.FrameModel, tea.Cmd) {
	if t, ok := msg.(TrainTickMsg); ok {
		m.done = t.Done || t.Err != nil
	}
	return m, nil
}

func (m *trainHintsModel) View(width, _ int) string {
	if m.done {
		return " q quit"
	}
	return " training in progress  │  ctrl-c cancel  │  q quit"
}

// --- Training Frame ---

// TrainFrame wraps a cli.Frame for training display.
// Usage example: tui := cmd.NewTrainFrame()
type TrainFrame struct {
	frame   *cli.Frame
	content *trainContentModel
	status  *trainStatusModel
	hints   *trainHintsModel
}

// NewTrainFrame creates a training dashboard TUI.
// Usage example: tui := cmd.NewTrainFrame()
func NewTrainFrame() *TrainFrame {
	status := newTrainStatusModel()
	content := newTrainContentModel()
	hints := newTrainHintsModel()

	frame := cli.NewFrame("HCF")
	frame.Header(status)
	frame.Content(content)
	frame.Footer(hints)

	return &TrainFrame{
		frame:   frame,
		content: content,
		status:  status,
		hints:   hints,
	}
}

// Run blocks, rendering the TUI until quit.
func (tf *TrainFrame) Run() {
	tf.frame.Run()
}

// Send injects a TrainTickMsg into the TUI.
func (tf *TrainFrame) Send(msg TrainTickMsg) {
	tf.frame.Send(msg)
}

// Stop signals the TUI to exit.
func (tf *TrainFrame) Stop() {
	tf.frame.Stop()
}

// SendTick is a convenience for sending periodic updates from the training goroutine.
func (tf *TrainFrame) SendTick(iter, total int, loss, valLoss, lr, tps, peakGB float64, tokens int64, phase, runID string) {
	tf.Send(TrainTickMsg{
		Iter:       iter,
		TotalIters: total,
		Loss:       loss,
		ValLoss:    valLoss,
		LR:         lr,
		TokensPerS: tps,
		PeakMemGB:  peakGB,
		Tokens:     tokens,
		Phase:      phase,
		RunID:      runID,
	})
}

// SendDone signals training completion.
func (tf *TrainFrame) SendDone(err error) {
	tf.Send(TrainTickMsg{Done: true, Err: err})
	// Give the TUI a moment to render the final state
	time.Sleep(100 * time.Millisecond)
}
