package ml

import "dappco.re/go"

func TestStatus_PrintStatus_Good(t *core.T) {
	queries := map[string][]map[string]any{
		"training_status": {{"model": "m", "status": "running", "iteration": float64(1), "total_iters": float64(2), "pct": float64(50)}},
		"training_loss":   {{"model": "m", "loss": float64(0.5)}},
	}
	influx, _ := newFakeInflux(t, queries, 0)
	buf := core.NewBuffer(nil)
	err := PrintStatus(influx, buf)
	core.RequireNoError(t, err)
	core.AssertContains(t, buf.String(), "running")
}

func TestStatus_PrintStatus_Bad(t *core.T) {
	influx := &InfluxClient{url: "http://127.0.0.1:1", db: "test"}
	buf := core.NewBuffer(nil)
	err := PrintStatus(influx, buf)
	core.RequireNoError(t, err)
	core.AssertContains(t, buf.String(), "no data")
}

func TestStatus_PrintStatus_Ugly(t *core.T) {
	influx, _ := newFakeInflux(t, nil, 0)
	buf := core.NewBuffer(nil)
	err := PrintStatus(influx, buf)
	core.RequireNoError(t, err)
	core.AssertContains(t, buf.String(), "Generation:")
}
