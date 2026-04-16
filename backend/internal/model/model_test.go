package model_test

import (
	"testing"

	"github.com/freewhen/backend/internal/model"
)

func TestCalendarWithDetailsDefaults(t *testing.T) {
	c := model.CalendarWithDetails{}
	if c.Calendar.ID != "" {
		t.Fatal("expected empty ID")
	}
	if c.Participants != nil {
		t.Fatal("expected nil participants")
	}
	if c.Availability != nil {
		t.Fatal("expected nil availability")
	}
}
