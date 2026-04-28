package asset

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/ydcloud-dy/opshub/pkg/response"
)

func TestTerminalAuditRecordingErrorsUseHTTPStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	handler := &TerminalAuditHandler{}
	handler.writeRecordingError(c, ErrTerminalRecordingMissing)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("unexpected status: got %d want %d", recorder.Code, http.StatusNotFound)
	}

	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if body.Code != http.StatusNotFound {
		t.Fatalf("unexpected response code: got %d want %d", body.Code, http.StatusNotFound)
	}
	if body.Message == "" {
		t.Fatal("expected response message")
	}
}
