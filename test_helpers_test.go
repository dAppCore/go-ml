// SPDX-Licence-Identifier: EUPL-1.2

package ml

import (
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
