package argus

import (
	"testing"
)

func TestWordLevMatchedASNs(t *testing.T) {
	a := []string{"ASN123", "ASN456", "ASN789"}
	b := []string{"ASN123", "ASN456", "ASN789"}
	result := WordLev(a, b)

	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
}

func TestWordLevOneChange(t *testing.T) {
	a := []string{"ASN123", "ASN456", "ASN789"}
	b := []string{"ASN123", "ASN999", "ASN789"}

	result := WordLev(a, b)
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}

func TestWordLevMultipleChanges(t *testing.T) {
	a := []string{"ASN123", "ASN456", "ASN789"}
	b := []string{"ASN000", "ASN999", "ASN789"}

	result := WordLev(a, b)
	if result != 2 {
		t.Errorf("Expected 2, got %d", result)
	}
}

func TestWordLevDifferentLengths(t *testing.T) {
	a := []string{"ASN123", "ASN456", "ASN789"}
	b := []string{"ASN123", "ASN456"}

	result := WordLev(a, b)
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}

func TestWordLevEmptySequences(t *testing.T) {
	a := []string{}
	b := []string{}

	result := WordLev(a, b)
	if result != 0 {
		t.Errorf("Expected 0, got %d", result)
	}
}

func TestWorldLevIPSequences(t *testing.T) {
	a := []string{"212.534.2.1", "212.534.2.2", "212.534.2.3"}
	b := []string{"212.534.2.1", "212.534.2.4", "212.534.2.3"}

	result := WordLev(a, b)
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}
}
