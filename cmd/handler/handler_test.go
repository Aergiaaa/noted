package handler

import (
	"testing"

	"github.com/Aergiaaa/noted/service"
)

func TestNew(t *testing.T) {
	svc := &service.Services{}
	h := New(svc)

	if h == nil {
		t.Fatalf("expected non-nil handler")
	}

	if h.Service != svc {
		t.Fatalf("expected service passthrough")
	}
}
