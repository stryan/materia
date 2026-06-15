package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNotifier(t *testing.T, triggers map[string]string) *Notifier {
	n, err := NewNotifier(NotifyConfig{Triggers: triggers})
	require.NoError(t, err)
	return n
}

func testHttpServer(t *testing.T) (*httptest.Server, func() []NotifyPayload) {
	t.Helper()
	var received []NotifyPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var p NotifyPayload
		require.NoError(t, json.Unmarshal(body, &p))
		received = append(received, p)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []NotifyPayload { return received }
}

func Test_Notify_NoTriggers(t *testing.T) {
	n := newNotifier(t, map[string]string{})
	err := n.Notify(context.Background(), NotifyUpdate, "msg")
	assert.NoError(t, err)
}

func Test_Notify_KnownEvent(t *testing.T) {
	srv, payloads := testHttpServer(t)

	n := newNotifier(t, map[string]string{
		NotifyUpdate: srv.URL,
	})
	err := n.Notify(context.Background(), NotifyUpdate, "update happened")
	require.NoError(t, err)

	got := payloads()
	require.Len(t, got, 1)
	assert.Equal(t, "update happened", got[0].Text)
}

func Test_Notify_UnknownTriggerFail(t *testing.T) {
	srv, payloads := testHttpServer(t)

	n := newNotifier(t, map[string]string{
		NotifyUpdate: srv.URL,
	})
	err := n.Notify(context.Background(), "somethingwickedthiswaycomes", "msg")
	assert.Error(t, err)
	assert.Empty(t, payloads())
}

func Test_Notify_FallbackDefault(t *testing.T) {
	srv, payloads := testHttpServer(t)

	n := newNotifier(t, map[string]string{
		NotifyDefault: srv.URL,
	})
	err := n.Notify(context.Background(), NotifyRollback, "rollback msg")
	require.NoError(t, err)

	got := payloads()
	require.Len(t, got, 1)
	assert.Equal(t, "rollback msg", got[0].Text)
}

func Test_NewNotifier_InvalidConfig(t *testing.T) {
	_, err := NewNotifier(NotifyConfig{Triggers: map[string]string{
		"foo": "https://example.com",
	}})
	assert.Error(t, err)
}
