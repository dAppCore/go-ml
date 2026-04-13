package ml

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"dappco.re/go/core"

	coreio "dappco.re/go/core/io"
	coreerr "dappco.re/go/core/log"
)

// InfluxClient talks to an InfluxDB v3 instance.
type InfluxClient struct {
	url   string
	db    string
	token string
}

// NewInfluxClient creates an InfluxClient for the given URL and database.
// Reads token from INFLUX_TOKEN env var first, then ~/.influx_token file.
// If url is empty, defaults to "http://10.69.69.165:8181".
// If db is empty, defaults to "training".
func NewInfluxClient(url, db string) *InfluxClient {
	if url == "" {
		url = "http://10.69.69.165:8181"
	}
	if db == "" {
		db = "training"
	}

	token := envGet("INFLUX_TOKEN")
	if token == "" {
		home, err := userHomeDir()
		if err == nil {
			data, err := coreio.Local.Read(core.Path(home, ".influx_token"))
			if err == nil {
				token = core.Trim(string(data))
			}
		}
	}

	return &InfluxClient{
		url:   url,
		db:    db,
		token: token,
	}
}

// WriteLp writes line protocol data to InfluxDB.
func (c *InfluxClient) WriteLp(lines []string) error {
	body := joinStrings(lines, "\n")

	url := core.Sprintf("%s/api/v3/write_lp?db=%s", c.url, c.db)

	req, err := http.NewRequest(http.MethodPost, url, core.NewReader(body))
	if err != nil {
		return coreerr.E("ml.InfluxClient.WriteLp", "create write request", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "text/plain")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return coreerr.E("ml.InfluxClient.WriteLp", "write request", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := readAll(resp.Body)
		return coreerr.E("ml.InfluxClient.WriteLp", core.Sprintf("write failed %d: %s", resp.StatusCode, string(respBody)), nil)
	}

	return nil
}

// QuerySQL runs a SQL query against InfluxDB and returns the result rows.
func (c *InfluxClient) QuerySQL(sql string) ([]map[string]any, error) {
	reqBody := map[string]string{
		"db": c.db,
		"q":  sql,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, coreerr.E("ml.InfluxClient.QuerySQL", "marshal query request", err)
	}

	url := c.url + "/api/v3/query_sql"

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, coreerr.E("ml.InfluxClient.QuerySQL", "create query request", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, coreerr.E("ml.InfluxClient.QuerySQL", "query request", err)
	}
	defer resp.Body.Close()

	respBody, err := readAll(resp.Body)
	if err != nil {
		return nil, coreerr.E("ml.InfluxClient.QuerySQL", "read query response", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, coreerr.E("ml.InfluxClient.QuerySQL", core.Sprintf("query failed %d: %s", resp.StatusCode, string(respBody)), nil)
	}

	var rows []map[string]any
	if err := json.Unmarshal(respBody, &rows); err != nil {
		return nil, coreerr.E("ml.InfluxClient.QuerySQL", "unmarshal query response", err)
	}

	return rows, nil
}

// EscapeLp escapes spaces, commas, and equals signs for InfluxDB line protocol
// tag values.
func EscapeLp(s string) string {
	s = replaceAll(s, `,`, `\,`)
	s = replaceAll(s, `=`, `\=`)
	s = replaceAll(s, ` `, `\ `)
	return s
}
