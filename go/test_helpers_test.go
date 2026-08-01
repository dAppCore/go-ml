// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
	"context"
	"io"
	"net/http"
	"testing"

	"dappco.re/go"
)

func mustJSONUnmarshalBytes(t testing.TB, data []byte, out any) {
	t.Helper()
	if r := core.JSONUnmarshal(data, out); !r.OK {
		t.Fatalf("unmarshal error: %v", r.Value.(error))
	}
}

func mustJSONUnmarshalString(t testing.TB, data string, out any) {
	t.Helper()
	if r := core.JSONUnmarshalString(data, out); !r.OK {
		t.Fatalf("unmarshal error: %v", r.Value.(error))
	}
}

func mustReadJSONRequest(t testing.TB, r *http.Request, out any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	mustJSONUnmarshalBytes(t, body, out)
}

func mustWriteJSONResponse(t testing.TB, w io.Writer, v any) {
	t.Helper()
	if _, err := io.WriteString(w, core.JSONMarshalString(v)); err != nil {
		t.Fatalf("write json response: %v", err)
	}
}

func requireResultOK(t testing.TB, r core.Result) {
	t.Helper()
	if !r.OK {
		t.Fatalf("unexpected result error: %s", r.Error())
	}
}

func assertResultOK(t testing.TB, r core.Result) {
	t.Helper()
	if !r.OK {
		t.Errorf("unexpected result error: %s", r.Error())
	}
}

func assertResultError(t testing.TB, r core.Result, contains ...string) {
	t.Helper()
	if r.OK {
		t.Fatalf("expected result error, got OK value %#v", r.Value)
	}
	if len(contains) > 0 && contains[0] != "" && !core.Contains(r.Error(), contains[0]) {
		t.Fatalf("expected result error containing %q, got %q", contains[0], r.Error())
	}
}

type testBackend struct {
	name      string
	available bool
	result    Result
	err       error
}

func (b *testBackend) Name() string {
	if b.name == "" {
		return "test"
	}
	return b.name
}

func (b *testBackend) Available() bool { return b.available }

func (b *testBackend) Generate(_ context.Context, prompt string, _ GenOpts) core.Result {
	if b.err != nil {
		return core.Fail(b.err)
	}
	if b.result.Text != "" {
		return core.Ok(b.result)
	}
	return core.Ok(Result{Text: prompt})
}

func (b *testBackend) Chat(_ context.Context, messages []Message, _ GenOpts) core.Result {
	if b.err != nil {
		return core.Fail(b.err)
	}
	if b.result.Text != "" {
		return core.Ok(b.result)
	}
	if len(messages) == 0 {
		return core.Ok(Result{})
	}
	return core.Ok(Result{Text: messages[len(messages)-1].Content})
}
