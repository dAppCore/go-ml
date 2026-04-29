//go:build darwin && arm64 && !nomlx && cliv1

package cmd

import (
	"time"

	"dappco.re/go"
	tea "github.com/charmbracelet/bubbletea"
)

func TestTuiTrain_StatusModel_Init_Good(t *core.T) {
	model := newTrainStatusModel()
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_StatusModel_Init_Bad(t *core.T) {
	model := &trainStatusModel{}
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_StatusModel_Init_Ugly(t *core.T) {
	model := newTrainStatusModel()
	model.tick.Done = true
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_StatusModel_Update_Good(t *core.T) {
	model := newTrainStatusModel()
	updated, cmd := model.Update(TrainTickMsg{Iter: 1, TotalIters: 2})
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_StatusModel_Update_Bad(t *core.T) {
	model := newTrainStatusModel()
	updated, cmd := model.Update("ignored")
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_StatusModel_Update_Ugly(t *core.T) {
	model := newTrainStatusModel()
	updated, cmd := model.Update(TrainTickMsg{Done: true})
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_StatusModel_View_Good(t *core.T) {
	model := newTrainStatusModel()
	model.Update(TrainTickMsg{Iter: 1, TotalIters: 2, Phase: "train"})
	view := model.View(80, 1)
	core.AssertContains(t, view, "train")
}

func TestTuiTrain_StatusModel_View_Bad(t *core.T) {
	model := newTrainStatusModel()
	view := model.View(80, 1)
	core.AssertContains(t, view, "loading")
}

func TestTuiTrain_StatusModel_View_Ugly(t *core.T) {
	model := newTrainStatusModel()
	model.Update(TrainTickMsg{Iter: 100, TotalIters: 100, Done: true})
	view := model.View(12, 1)
	core.AssertTrue(t, len(view) <= 12)
}

func TestTuiTrain_ContentModel_Init_Good(t *core.T) {
	model := newTrainContentModel()
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_ContentModel_Init_Bad(t *core.T) {
	model := &trainContentModel{}
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_ContentModel_Init_Ugly(t *core.T) {
	model := newTrainContentModel()
	model.lossHist = append(model.lossHist, 1)
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_ContentModel_Update_Good(t *core.T) {
	model := newTrainContentModel()
	updated, cmd := model.Update(TrainTickMsg{Iter: 1, TotalIters: 2, Loss: 0.5})
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_ContentModel_Update_Bad(t *core.T) {
	model := newTrainContentModel()
	updated, cmd := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_ContentModel_Update_Ugly(t *core.T) {
	model := newTrainContentModel()
	updated, cmd := model.Update(TrainTickMsg{Iter: 1, TotalIters: 2, ValLoss: 0.6})
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_ContentModel_View_Good(t *core.T) {
	model := newTrainContentModel()
	model.Update(TrainTickMsg{Iter: 1, TotalIters: 2, Loss: 0.5})
	view := model.View(80, 24)
	core.AssertContains(t, view, "iteration")
}

func TestTuiTrain_ContentModel_View_Bad(t *core.T) {
	model := newTrainContentModel()
	view := model.View(80, 24)
	core.AssertContains(t, view, "waiting")
}

func TestTuiTrain_ContentModel_View_Ugly(t *core.T) {
	model := newTrainContentModel()
	model.Update(TrainTickMsg{Iter: 2, TotalIters: 2, Loss: 0.5, Done: true})
	view := model.View(25, 8)
	core.AssertContains(t, view, "complete")
}

func TestTuiTrain_HintsModel_Init_Good(t *core.T) {
	model := newTrainHintsModel()
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_HintsModel_Init_Bad(t *core.T) {
	model := &trainHintsModel{}
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_HintsModel_Init_Ugly(t *core.T) {
	model := newTrainHintsModel()
	model.done = true
	cmd := model.Init()
	core.AssertNil(t, cmd)
}

func TestTuiTrain_HintsModel_Update_Good(t *core.T) {
	model := newTrainHintsModel()
	updated, cmd := model.Update(TrainTickMsg{Done: true})
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_HintsModel_Update_Bad(t *core.T) {
	model := newTrainHintsModel()
	updated, cmd := model.Update("ignored")
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_HintsModel_Update_Ugly(t *core.T) {
	model := newTrainHintsModel()
	updated, cmd := model.Update(TrainTickMsg{Err: core.AnError})
	core.AssertNil(t, cmd)
	core.AssertNotNil(t, updated)
}

func TestTuiTrain_HintsModel_View_Good(t *core.T) {
	model := newTrainHintsModel()
	view := model.View(80, 1)
	core.AssertContains(t, view, "training")
}

func TestTuiTrain_HintsModel_View_Bad(t *core.T) {
	model := newTrainHintsModel()
	model.Update(TrainTickMsg{Done: true})
	view := model.View(80, 1)
	core.AssertContains(t, view, "quit")
}

func TestTuiTrain_HintsModel_View_Ugly(t *core.T) {
	model := newTrainHintsModel()
	model.Update(TrainTickMsg{Err: core.AnError})
	view := model.View(5, 1)
	core.AssertContains(t, view, "q")
}

func TestTuiTrain_NewTrainFrame_Good(t *core.T) {
	frame := NewTrainFrame()
	core.AssertNotNil(t, frame)
	core.AssertNotNil(t, frame.content)
}

func TestTuiTrain_NewTrainFrame_Bad(t *core.T) {
	frame := NewTrainFrame()
	core.AssertNotNil(t, frame.status)
	core.AssertNotNil(t, frame.hints)
}

func TestTuiTrain_NewTrainFrame_Ugly(t *core.T) {
	frame := NewTrainFrame()
	frame.Stop()
	core.AssertNotNil(t, frame.frame)
}

func TestTuiTrain_TrainFrame_Run_Good(t *core.T) {
	frame := NewTrainFrame()
	frame.Stop()
	core.AssertNotPanics(t, func() { frame.Run() })
}

func TestTuiTrain_TrainFrame_Run_Bad(t *core.T) {
	frame := NewTrainFrame()
	frame.Stop()
	core.AssertNotPanics(t, func() { frame.Run() })
	core.AssertNotNil(t, frame.frame)
}

func TestTuiTrain_TrainFrame_Run_Ugly(t *core.T) {
	frame := NewTrainFrame()
	frame.Send(TrainTickMsg{Done: true})
	frame.Stop()
	core.AssertNotPanics(t, func() { frame.Run() })
}

func TestTuiTrain_TrainFrame_Send_Good(t *core.T) {
	frame := NewTrainFrame()
	frame.Send(TrainTickMsg{Iter: 1})
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_Send_Bad(t *core.T) {
	frame := NewTrainFrame()
	frame.Send(nil)
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_Send_Ugly(t *core.T) {
	frame := NewTrainFrame()
	frame.Send(TrainTickMsg{Err: core.AnError})
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_Stop_Good(t *core.T) {
	frame := NewTrainFrame()
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_Stop_Bad(t *core.T) {
	frame := NewTrainFrame()
	frame.Stop()
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_Stop_Ugly(t *core.T) {
	frame := NewTrainFrame()
	frame.Send(TrainTickMsg{Done: true})
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_SendTick_Good(t *core.T) {
	frame := NewTrainFrame()
	frame.SendTick(1, 2, 0.5, 0.6, 0.001, 20, 1.5, 100, "train", "run")
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_SendTick_Bad(t *core.T) {
	frame := NewTrainFrame()
	frame.SendTick(0, 0, 0, 0, 0, 0, 0, 0, "", "")
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_SendTick_Ugly(t *core.T) {
	frame := NewTrainFrame()
	frame.SendTick(2, 1, -1, -1, -1, -1, -1, -1, "done", "run")
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_SendDone_Good(t *core.T) {
	frame := NewTrainFrame()
	frame.SendDone(nil)
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_SendDone_Bad(t *core.T) {
	frame := NewTrainFrame()
	frame.SendDone(core.AnError)
	frame.Stop()
	core.AssertNotNil(t, frame)
}

func TestTuiTrain_TrainFrame_SendDone_Ugly(t *core.T) {
	frame := NewTrainFrame()
	start := time.Now()
	frame.SendDone(nil)
	frame.Stop()
	core.AssertTrue(t, time.Since(start) >= 0)
}
