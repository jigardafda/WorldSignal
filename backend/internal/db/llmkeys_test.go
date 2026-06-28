package db_test

import (
	"context"
	"testing"

	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
)

func TestLLMKeyStore(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	model := "gpt-4o-mini"
	by := "admin"

	k1, err := d.CreateLLMKey(ctx, cuid.New(), db.CreateLLMKeyInput{Provider: "OPENAI", Label: "First", Ciphertext: "ct1", Last4: "aaaa", Model: &model, CreatedBy: &by})
	if err != nil || k1.IsActive || k1.Status != "UNTESTED" {
		t.Fatalf("create k1: %+v err=%v", k1, err)
	}
	k2, err := d.CreateLLMKey(ctx, cuid.New(), db.CreateLLMKeyInput{Provider: "OPENAI", Label: "Second", Ciphertext: "ct2", Last4: "bbbb"})
	if err != nil {
		t.Fatal(err)
	}

	// No active key yet.
	if a, _ := d.GetActiveLLMKey(ctx, "OPENAI"); a != nil {
		t.Fatal("expected no active key")
	}

	// Activate k1, then k2 → only k2 active.
	if _, err := d.SetActiveLLMKey(ctx, k1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := d.SetActiveLLMKey(ctx, k2.ID); err != nil {
		t.Fatal(err)
	}
	active, _ := d.GetActiveLLMKey(ctx, "OPENAI")
	if active == nil || active.ID != k2.ID {
		t.Fatalf("expected k2 active, got %+v", active)
	}
	all, _ := d.ListLLMKeys(ctx)
	activeCount := 0
	for _, k := range all {
		if k.IsActive {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Fatalf("exactly one key should be active, got %d", activeCount)
	}

	// Status update.
	msg := "provider rejected key (HTTP 401)"
	if err := d.UpdateLLMKeyStatus(ctx, k1.ID, "INVALID", &msg); err != nil {
		t.Fatal(err)
	}
	got, _ := d.GetLLMKey(ctx, k1.ID)
	if got.Status != "INVALID" || got.LastError == nil || *got.LastError != msg || got.LastTestedAt == nil {
		t.Fatalf("status not updated: %+v", got)
	}

	// SetActive on unknown id → (nil,nil).
	if k, err := d.SetActiveLLMKey(ctx, "missing"); err != nil || k != nil {
		t.Fatalf("setactive missing: k=%v err=%v", k, err)
	}
	// GetLLMKey unknown → (nil,nil).
	if k, err := d.GetLLMKey(ctx, "missing"); err != nil || k != nil {
		t.Fatalf("get missing: k=%v err=%v", k, err)
	}

	// Delete.
	ok, err := d.DeleteLLMKey(ctx, k2.ID)
	if err != nil || !ok {
		t.Fatalf("delete k2: ok=%v err=%v", ok, err)
	}
	if ok, _ := d.DeleteLLMKey(ctx, k2.ID); ok {
		t.Fatal("second delete should report false")
	}
	if a, _ := d.GetActiveLLMKey(ctx, "OPENAI"); a != nil {
		t.Fatal("active key removed with delete")
	}
}

// TestLLMKeyDBErrors covers the DB-error branch of each store function by hiding
// the table (rename) while keeping a valid id reference.
func TestLLMKeyDBErrors(t *testing.T) {
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	ctx := context.Background()
	k, err := d.CreateLLMKey(ctx, cuid.New(), db.CreateLLMKeyInput{Provider: "OPENAI", Label: "X", Ciphertext: "c", Last4: "zzzz"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "LLMKey" RENAME TO "LLMKey__h"`); err != nil {
		t.Fatal(err)
	}
	defer d.Pool.Exec(ctx, `ALTER TABLE "LLMKey__h" RENAME TO "LLMKey"`)

	if _, err := d.ListLLMKeys(ctx); err == nil {
		t.Fatal("ListLLMKeys should error")
	}
	if _, err := d.GetLLMKey(ctx, k.ID); err == nil {
		t.Fatal("GetLLMKey should error")
	}
	if _, err := d.GetActiveLLMKey(ctx, "OPENAI"); err == nil {
		t.Fatal("GetActiveLLMKey should error")
	}
	if _, err := d.CreateLLMKey(ctx, cuid.New(), db.CreateLLMKeyInput{Provider: "OPENAI", Label: "Y", Ciphertext: "c", Last4: "yyyy"}); err == nil {
		t.Fatal("CreateLLMKey should error")
	}
	if _, err := d.SetActiveLLMKey(ctx, k.ID); err == nil {
		t.Fatal("SetActiveLLMKey should error")
	}
	if err := d.UpdateLLMKeyStatus(ctx, k.ID, "VALID", nil); err == nil {
		t.Fatal("UpdateLLMKeyStatus should error")
	}
	if _, err := d.DeleteLLMKey(ctx, k.ID); err == nil {
		t.Fatal("DeleteLLMKey should error")
	}
}
