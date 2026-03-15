package netnormalize

import "testing"

func TestNormalizeMAC(t *testing.T) {
	t.Parallel()

	bare, colon, err := NormalizeMAC("AA-BB-CC-DD-EE-FF")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if bare != "aabbccddeeff" {
		t.Fatalf("expected bare normalized mac, got %q", bare)
	}
	if colon != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("expected colon normalized mac, got %q", colon)
	}
}

func TestNormalizeMACRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	if _, _, err := NormalizeMAC("bad-mac"); err == nil {
		t.Fatal("expected invalid MAC to fail")
	}
}
