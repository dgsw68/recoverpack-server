package auth

import "testing"

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct-horse-battery-staple" {
		t.Fatal("password was stored as plaintext")
	}
	if !VerifyPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("correct password did not verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("wrong password verified")
	}
}

func TestHashPasswordRejectsShortPassword(t *testing.T) {
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password should be rejected")
	}
}
