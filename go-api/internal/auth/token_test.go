package auth

import (
	"strings"
	"testing"
)

func TestCreateAndParseToken(t *testing.T) {
	t.Setenv("AUTH_SECRET", strings.Repeat("s", 32))
	token, err := CreateToken("user-1", "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ParseToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-1" || claims.Email != "user@example.com" {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestParseTokenRejectsTampering(t *testing.T) {
	t.Setenv("AUTH_SECRET", strings.Repeat("s", 32))
	token, err := CreateToken("user-1", "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseToken(token + "tampered"); err == nil {
		t.Fatal("tampered token should be rejected")
	}
}
