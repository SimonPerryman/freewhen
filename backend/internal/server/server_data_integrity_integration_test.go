//go:build integration

package server_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/freewhen/backend/internal/handler"
	"github.com/freewhen/backend/internal/idprovider"
	"github.com/freewhen/backend/internal/limits"
	"github.com/freewhen/backend/internal/model"
	"github.com/freewhen/backend/internal/service"
	"github.com/freewhen/backend/internal/store"
)

// Regression tests for plans/tech-debt/service-data-integrity.md.

// Two (or more) first-time sign-ins for the same address racing each other
// used to 500: both missed the lookup, both inserted, and the loser's
// users.email unique violation was mistaken for an id collision and retried
// until it gave up. Now the loser resolves to the winner's row.
func TestConcurrentFirstSignIn_SameEmailResolvesToOneUser(t *testing.T) {
	setupTestServer(t) // migrates + cleans tables
	db := openTestDB(t)
	accounts := service.NewAccounts(store.New(db))

	const n = 12
	start := make(chan struct{})
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			var u model.User
			var err error
			if i%2 == 0 {
				u, err = accounts.FindOrCreateOAuthUser(context.Background(), "google",
					idprovider.Identity{Sub: "race-sub", Email: "race@example.com", Name: "Racer", EmailVerified: true})
			} else {
				u, err = accounts.FindOrCreateByEmail(context.Background(), "race@example.com", "", "email", "")
			}
			ids[i], errs[i] = u.ID, err
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("sign-in %d failed: %v", i, err)
		}
		if ids[i] == "" || ids[i] != ids[0] {
			t.Fatalf("sign-in %d resolved to %q, want %q (one shared user)", i, ids[i], ids[0])
		}
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = 'race@example.com'`).Scan(&count); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("users with the raced email = %d, want 1", count)
	}
}

func TestCreateUser_DuplicateEmailIsErrUserExists(t *testing.T) {
	setupTestServer(t)
	st := store.New(openTestDB(t))
	if _, err := st.CreateUser(t.Context(), "dupe@example.com", "A", "email", ""); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := st.CreateUser(t.Context(), "dupe@example.com", "B", "google", "sub"); !errors.Is(err, model.ErrUserExists) {
		t.Fatalf("second create err = %v, want ErrUserExists", err)
	}
}

// PATCH with name + email used to be two writes: the rename committed even
// when the email then failed validation (or the second write failed).
func TestRenameParticipant_NameAndEmailAreAtomic(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Atomic")
	alice, aliceTok := joinAnon(t, ts, cal.ID, map[string]string{"name": "Alice", "email": "alice@example.com"})
	joinAnon(t, ts, cal.ID, map[string]string{"name": "Bob"})
	db := openTestDB(t)
	path := "/calendars/" + cal.ID + "/participants/" + alice.ID
	hdr := map[string]string{handler.ParticipantTokenHeader: aliceTok}

	readRow := func() (string, *string) {
		t.Helper()
		var name string
		var email *string
		if err := db.QueryRow(`SELECT name, email FROM participants WHERE id = $1`, alice.ID).Scan(&name, &email); err != nil {
			t.Fatalf("read participant: %v", err)
		}
		return name, email
	}

	// Invalid email: nothing is written, including the name.
	resp, body := doJSONRaw(t, ts, http.MethodPatch, path, map[string]string{"name": "Alicia", "email": "not-an-email"}, "", hdr)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid email: %d %s", resp.StatusCode, body)
	}
	if name, email := readRow(); name != "Alice" || email == nil || *email != "alice@example.com" {
		t.Fatalf("after rejected PATCH: name=%q email=%v, want unchanged", name, email)
	}

	// Name conflict: the email must not be written either.
	resp, body = doJSONRaw(t, ts, http.MethodPatch, path, map[string]string{"name": "Bob", "email": "new@example.com"}, "", hdr)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate name: %d %s", resp.StatusCode, body)
	}
	if name, email := readRow(); name != "Alice" || email == nil || *email != "alice@example.com" {
		t.Fatalf("after conflicting PATCH: name=%q email=%v, want unchanged", name, email)
	}

	// Happy path: both change together.
	resp, body = doJSONRaw(t, ts, http.MethodPatch, path, map[string]string{"name": "Alicia", "email": "Alicia@Example.com "}, "", hdr)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename+email: %d %s", resp.StatusCode, body)
	}
	var p model.Participant
	decodeJSON(t, body, &p)
	if p.Name != "Alicia" || p.Email == nil || *p.Email != "alicia@example.com" {
		t.Fatalf("response = %+v, want name Alicia + normalised email", p)
	}
	if name, email := readRow(); name != "Alicia" || email == nil || *email != "alicia@example.com" {
		t.Fatalf("stored name=%q email=%v", name, email)
	}

	// Empty email clears it in the same write.
	resp, body = doJSONRaw(t, ts, http.MethodPatch, path, map[string]string{"name": "Alicia B", "email": ""}, "", hdr)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("rename+clear: %d %s", resp.StatusCode, body)
	}
	if name, email := readRow(); name != "Alicia B" || email != nil {
		t.Fatalf("stored name=%q email=%v, want cleared email", name, email)
	}
}

// The finalised check used to run before the availability transaction, so a
// finalise committing in between was missed and availability was written to
// a finalised calendar. It now runs inside the tx under a share lock on the
// calendar row: an in-flight finalise blocks the write, which then sees the
// finalised calendar and is rejected.
func TestAvailabilityWrites_BlockOnInFlightFinaliseThenReject(t *testing.T) {
	setupTestServer(t)
	db := openTestDB(t)
	st := store.New(db)
	ctx := context.Background()

	owner, err := st.CreateUser(ctx, "owner-race@example.com", "Owner", "google", "owner-race")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	cal, err := st.CreateCalendar(ctx, "Race", "", owner.ID)
	if err != nil {
		t.Fatalf("create calendar: %v", err)
	}
	p, err := st.AddParticipant(ctx, cal.ID, "Alice", nil, nil)
	if err != nil {
		t.Fatalf("add participant: %v", err)
	}
	date := time.Now().UTC().Format("2006-01-02")

	writes := map[string]func() error{
		"SetAvailability": func() error {
			return st.SetAvailability(ctx, p.ID, []string{date}, nil, nil)
		},
		"MergeAvailabilityDates": func() error {
			return st.MergeAvailabilityDates(ctx, p.ID, []string{date})
		},
	}
	for name, write := range writes {
		t.Run(name, func(t *testing.T) {
			if _, err := db.Exec(`UPDATE calendars SET finalised_date = NULL, finalised_at = NULL WHERE id = $1`, cal.ID); err != nil {
				t.Fatalf("reset: %v", err)
			}

			// Hold an uncommitted finalise.
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatalf("begin: %v", err)
			}
			defer func() { _ = tx.Rollback() }()
			if _, err := tx.Exec(`UPDATE calendars SET finalised_date = $1, finalised_at = now() WHERE id = $2`, date, cal.ID); err != nil {
				t.Fatalf("finalise in tx: %v", err)
			}

			done := make(chan error, 1)
			go func() { done <- write() }()

			select {
			case err := <-done:
				t.Fatalf("write returned (%v) while a finalise was in flight; want it to wait on the calendar lock", err)
			case <-time.After(300 * time.Millisecond):
			}

			if err := tx.Commit(); err != nil {
				t.Fatalf("commit finalise: %v", err)
			}
			select {
			case err := <-done:
				if !errors.Is(err, model.ErrCalendarFinalised) {
					t.Fatalf("write err = %v, want ErrCalendarFinalised", err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("write still blocked after finalise committed")
			}

			var rows int
			if err := db.QueryRow(`SELECT COUNT(*) FROM availability WHERE participant_id = $1`, p.ID).Scan(&rows); err != nil {
				t.Fatalf("count availability: %v", err)
			}
			if rows != 0 {
				t.Fatalf("availability rows on finalised calendar = %d, want 0", rows)
			}
		})
	}
}

// Since #74 anonymous participants aren't copied, so they must not count
// towards the duplicate's participant cap.
func TestDuplicateCalendar_AnonymousParticipantsDontCountTowardsCap(t *testing.T) {
	ts := setupTestServer(t)
	cal, _ := createCalendarAuth(t, ts, "Full of anons")
	st := store.New(openTestDB(t))
	// Owner + (cap-1) anonymous joiners = exactly the cap.
	for i := 0; i < limits.MaxParticipantsPerCalendar-1; i++ {
		if _, err := st.AddParticipant(t.Context(), cal.ID, fmt.Sprintf("Anon %d", i), nil, nil); err != nil {
			t.Fatalf("add anon %d: %v", i, err)
		}
	}

	resp, body := doJSONAuth(t, ts, http.MethodPost, "/calendars/"+cal.ID+"/duplicate", map[string]any{"include_participants": true})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("duplicate: %d %s (anonymous rows aren't copied, so the copy fits)", resp.StatusCode, body)
	}
}
