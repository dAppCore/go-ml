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

func (b *testBackend) Generate(_ context.Context, prompt string, _ GenOpts) (Result, error) {
	if b.err != nil {
		return Result{}, b.err
	}
	if b.result.Text != "" {
		return b.result, nil
	}
	return Result{Text: prompt}, nil
}

func (b *testBackend) Chat(_ context.Context, messages []Message, _ GenOpts) (Result, error) {
	if b.err != nil {
		return Result{}, b.err
	}
	if b.result.Text != "" {
		return b.result, nil
	}
	if len(messages) == 0 {
		return Result{}, nil
	}
	return Result{Text: messages[len(messages)-1].Content}, nil
}
