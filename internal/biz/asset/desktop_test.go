package asset

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ydcloud-dy/opshub/internal/conf"
)

type fakeDesktopSessionRepo struct {
	sessions        map[uint]*DesktopSession
	touchedID       uint
	touchedAt       time.Time
	staleSessions   []*DesktopSession
	lastStaleCutoff time.Time
	updated         []*DesktopSession
}

func (r *fakeDesktopSessionRepo) Create(ctx context.Context, session *DesktopSession) error {
	if r.sessions == nil {
		r.sessions = make(map[uint]*DesktopSession)
	}
	r.sessions[session.ID] = session
	return nil
}

func (r *fakeDesktopSessionRepo) Update(ctx context.Context, session *DesktopSession) error {
	r.updated = append(r.updated, session)
	if r.sessions != nil {
		r.sessions[session.ID] = session
	}
	return nil
}

func (r *fakeDesktopSessionRepo) Touch(ctx context.Context, id uint, at time.Time) error {
	r.touchedID = id
	r.touchedAt = at
	return nil
}

func (r *fakeDesktopSessionRepo) Delete(ctx context.Context, id uint) error {
	delete(r.sessions, id)
	return nil
}

func (r *fakeDesktopSessionRepo) GetByID(ctx context.Context, id uint) (*DesktopSession, error) {
	if session, ok := r.sessions[id]; ok {
		return session, nil
	}
	return nil, errors.New("not found")
}

func (r *fakeDesktopSessionRepo) GetBySessionUUID(ctx context.Context, sessionUUID string) (*DesktopSession, error) {
	for _, session := range r.sessions {
		if session.SessionUUID == sessionUUID {
			return session, nil
		}
	}
	return nil, errors.New("not found")
}

func (r *fakeDesktopSessionRepo) List(ctx context.Context, page, pageSize int, keyword, status string, userID uint) ([]*DesktopSession, int64, error) {
	return nil, 0, nil
}

func (r *fakeDesktopSessionRepo) ListStaleOpen(ctx context.Context, cutoff time.Time, limit int) ([]*DesktopSession, error) {
	r.lastStaleCutoff = cutoff
	return r.staleSessions, nil
}

func TestDesktopSessionHeartbeatTouchesOpenSession(t *testing.T) {
	repo := &fakeDesktopSessionRepo{
		sessions: map[uint]*DesktopSession{
			11: {ID: 11, UserID: 7, Status: "active"},
		},
	}
	uc := NewDesktopSessionUseCase(nil, nil, repo, conf.DesktopConfig{})

	if err := uc.Heartbeat(context.Background(), 11, 7); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}
	if repo.touchedID != 11 {
		t.Fatalf("Touch id = %d, want 11", repo.touchedID)
	}
	if repo.touchedAt.IsZero() {
		t.Fatalf("Touch time was not set")
	}
}

func TestDesktopSessionHeartbeatRejectsOtherUser(t *testing.T) {
	repo := &fakeDesktopSessionRepo{
		sessions: map[uint]*DesktopSession{
			11: {ID: 11, UserID: 7, Status: "active"},
		},
	}
	uc := NewDesktopSessionUseCase(nil, nil, repo, conf.DesktopConfig{})

	if err := uc.Heartbeat(context.Background(), 11, 8); err == nil {
		t.Fatalf("Heartbeat() expected permission error")
	}
	if repo.touchedID != 0 {
		t.Fatalf("Touch should not be called, got id %d", repo.touchedID)
	}
}

func TestReconcileStaleOpenSessionsMarksTimeout(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	updatedAt := now.Add(-5 * time.Minute)
	session := &DesktopSession{ID: 22, UserID: 7, Status: "active", UpdatedAt: updatedAt}
	repo := &fakeDesktopSessionRepo{staleSessions: []*DesktopSession{session}}
	uc := NewDesktopSessionUseCase(nil, nil, repo, conf.DesktopConfig{})

	closed, err := uc.ReconcileStaleOpenSessions(context.Background(), now)
	if err != nil {
		t.Fatalf("ReconcileStaleOpenSessions() error = %v", err)
	}
	if closed != 1 {
		t.Fatalf("closed = %d, want 1", closed)
	}
	if session.Status != "timeout" {
		t.Fatalf("status = %q, want timeout", session.Status)
	}
	if session.CloseReason != "heartbeat_timeout" {
		t.Fatalf("close reason = %q, want heartbeat_timeout", session.CloseReason)
	}
	wantEndedAt := updatedAt.Add(desktopSessionStaleAfter)
	if session.EndedAt == nil || !session.EndedAt.Equal(wantEndedAt) {
		t.Fatalf("ended_at = %v, want %v", session.EndedAt, wantEndedAt)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("updated count = %d, want 1", len(repo.updated))
	}
	if !repo.lastStaleCutoff.Equal(now.Add(-desktopSessionStaleAfter)) {
		t.Fatalf("cutoff = %v, want %v", repo.lastStaleCutoff, now.Add(-desktopSessionStaleAfter))
	}
}
