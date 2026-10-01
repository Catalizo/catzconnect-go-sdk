package catzconnect

import "testing"

// The same vector the server's own test uses (services/push/devices).
const userHashVector = "22182382240b0a5afc85fca04d38b5c9e519ecb9a540c5ff50785e7d73b60a2c"

func TestComputeUserHash(t *testing.T) {
	if got := ComputeUserHash("user-42", "pis_test"); got != userHashVector {
		t.Fatalf("user hash = %s, want %s", got, userHashVector)
	}
	if ComputeUserHash("user-43", "pis_test") == userHashVector {
		t.Fatal("different user produced the same hash")
	}
}
