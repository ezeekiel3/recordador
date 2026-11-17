package main

import (
	"os"
	"testing"
	"time"
)

func TestStoreOperations(t *testing.T) {
	testFile := "test_recordatorios.json"
	defer os.Remove(testFile)

	store, err := NewStore(testFile)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// 1. Create reminder
	due := time.Now().Add(10 * time.Minute)
	rem, err := store.Create("Comprar leche", "Descremada y sin lactosa", due, "campana")
	if err != nil {
		t.Fatalf("Failed to create reminder: %v", err)
	}
	if rem.Title != "Comprar leche" {
		t.Errorf("Expected title 'Comprar leche', got '%s'", rem.Title)
	}
	if rem.Text != "Descremada y sin lactosa" {
		t.Errorf("Expected text 'Descremada y sin lactosa', got '%s'", rem.Text)
	}

	// 2. Read
	all := store.GetAll()
	if len(all) != 1 {
		t.Fatalf("Expected 1 reminder, got %d", len(all))
	}

	// 3. Update / Edit
	newDue := time.Now().Add(20 * time.Minute)
	updated, err := store.Update(rem.ID, "Comprar leche y pan", "Pan integral", newDue, "digital")
	if err != nil {
		t.Fatalf("Failed to update reminder: %v", err)
	}
	if updated.Title != "Comprar leche y pan" || updated.Text != "Pan integral" {
		t.Errorf("Update did not persist title/text correctly")
	}

	// 4. Toggle Complete
	completed, err := store.ToggleComplete(rem.ID)
	if err != nil || !completed.Completed {
		t.Fatalf("Failed to toggle complete")
	}

	// Reactivate
	reactivated, err := store.ToggleComplete(rem.ID)
	if err != nil || reactivated.Completed {
		t.Fatalf("Failed to reactivate reminder")
	}

	// 5. Snooze
	snoozed, err := store.Snooze(rem.ID, 15)
	if err != nil {
		t.Fatalf("Failed to snooze: %v", err)
	}
	if snoozed.DueTime.Before(time.Now().Add(14 * time.Minute)) {
		t.Errorf("Snooze time invalid: %v", snoozed.DueTime)
	}

	// 6. Alarm triggering test
	// Create an overdue reminder
	pastDue := time.Now().Add(-1 * time.Minute)
	alarmRem, err := store.Create("Alarma urgente", "Debe sonar ya", pastDue, "campana")
	if err != nil {
		t.Fatalf("Failed to create urgent reminder: %v", err)
	}

	triggeredList := store.CheckDue()
	found := false
	for _, it := range triggeredList {
		if it.ID == alarmRem.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected alarmRem to trigger in CheckDue")
	}

	// Check active alarms
	active := store.GetActiveAlarms()
	if len(active) == 0 {
		t.Errorf("Expected active alarms to have at least 1 item")
	}

	// Dismiss alarm
	_, err = store.DismissAlarm(alarmRem.ID)
	if err != nil {
		t.Fatalf("Failed to dismiss alarm: %v", err)
	}

	// 7. Delete
	err = store.Delete(rem.ID)
	if err != nil {
		t.Fatalf("Failed to delete reminder: %v", err)
	}

	allAfterDel := store.GetAll()
	if len(allAfterDel) != 1 { // only alarmRem remains
		t.Errorf("Expected 1 reminder left, got %d", len(allAfterDel))
	}
}
