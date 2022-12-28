package elasticsearch

import "net/http"

type mockTransport struct {
	resp *http.Response
	err  error
}

func (m *mockTransport) Perform(*http.Request) (*http.Response, error) {
	return m.resp, m.err
}
