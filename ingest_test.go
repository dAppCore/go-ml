package ml

import (
	"dappco.re/go"
	coreio "dappco.re/go/io"
)

func TestIngest_Ingest_Good(t *core.T) {
	logFile := core.JoinPath(t.TempDir(), "train.log")
	core.RequireNoError(t, coreio.Local.Write(logFile, "Iter 1: Train loss 0.5, Learning Rate 1e-5, It/sec 2.0, Tokens/sec 30.0\n"))
	influx, rec := newFakeInflux(t, nil, 0)
	err := Ingest(influx, IngestConfig{TrainingLog: logFile, Model: "m", BatchSize: 1}, core.NewBuffer(nil))
	core.RequireNoError(t, err)
	core.AssertEqual(t, 1, rec.writeCount())
}

func TestIngest_Ingest_Bad(t *core.T) {
	influx, _ := newFakeInflux(t, nil, 0)
	err := Ingest(influx, IngestConfig{}, core.NewBuffer(nil))
	core.AssertError(t, err)
}

func TestIngest_Ingest_Ugly(t *core.T) {
	contentFile := core.JoinPath(t.TempDir(), "content.out")
	core.RequireNoError(t, coreio.Local.Write(contentFile, "not object\n"))
	influx, _ := newFakeInflux(t, nil, 0)
	err := Ingest(influx, IngestConfig{ContentFile: contentFile, Model: "m"}, core.NewBuffer(nil))
	core.AssertError(t, err)
}
