package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	logrus.SetOutput(io.Discard)
	os.Exit(m.Run())
}

func newTestRouter() *gin.Engine {
	r := gin.New()
	r.POST("/execute", executeHandler)
	return r
}

type closeNotifyingRecorder struct {
	*httptest.ResponseRecorder
	closeCh chan bool
}

func newCloseNotifyingRecorder() *closeNotifyingRecorder {
	return &closeNotifyingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeCh:          make(chan bool, 1),
	}
}

func (r *closeNotifyingRecorder) CloseNotify() <-chan bool {
	return r.closeCh
}

func performRequest(r *gin.Engine, body []byte) *closeNotifyingRecorder {
	req := httptest.NewRequest(http.MethodPost, "/execute", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := newCloseNotifyingRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestExecuteHandler_BadRequest(t *testing.T) {
	whitelistEnabled = false
	whitelist = nil

	r := newTestRouter()
	w := performRequest(r, []byte(`{"args":["-lah"]}`))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestExecuteHandler_WhitelistForbidden(t *testing.T) {
	whitelistEnabled = true
	whitelist = map[string]bool{"allowed": true}

	r := newTestRouter()
	payload, err := json.Marshal(Request{Cmd: "denied"})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	w := performRequest(r, payload)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestExecuteHandler_Success(t *testing.T) {
	whitelistEnabled = false
	whitelist = nil
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	r := newTestRouter()
	payload, err := json.Marshal(Request{
		Cmd:  os.Args[0],
		Args: []string{"-test.run=TestHelperProcess", "--", "hello"},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	w := performRequest(r, payload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "out:hello") {
		t.Fatalf("expected stdout in body, got %q", body)
	}
	if !strings.Contains(body, "err:hello") {
		t.Fatalf("expected stderr in body, got %q", body)
	}
}

func TestExecuteHandler_WhitelistAllowed(t *testing.T) {
	whitelistEnabled = true
	whitelist = map[string]bool{os.Args[0]: true}
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")

	r := newTestRouter()
	payload, err := json.Marshal(Request{
		Cmd:  os.Args[0],
		Args: []string{"-test.run=TestHelperProcess", "--", "allowed"},
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	w := performRequest(r, payload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
}

// TestHelperProcess is used as a portable subprocess for exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	msg := "default"
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			msg = os.Args[i+1]
			break
		}
	}

	fmt.Fprintf(os.Stdout, "out:%s\n", msg)
	fmt.Fprintf(os.Stderr, "err:%s\n", msg)
}
