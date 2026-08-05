//go:build darwin || linux

package transport

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCredentialFingerprintAndControlByteRefusals(t *testing.T) {
	t.Parallel()
	cred, err := NewCredential("plain-key", "plain-secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	if got, want := cred.Fingerprint(), "2b5cde0108a957d90d131d664db092b0b604ee9741ef89e937ebc526be94cc57"; got != want {
		t.Fatalf("Fingerprint = %q, want %q", got, want)
	}

	tests := []struct {
		name   string
		key    string
		secret string
		field  string
		offset string
		value  string
	}{
		{"key LF", "abc\ndef", "secret", "api key", "3", "abc\ndef"},
		{"secret NUL", "key", "abc\x00def", "secret", "3", "abc\x00def"},
		{"key DEL", "ab\x7fcd", "secret", "api key", "2", "ab\x7fcd"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCredential(tc.key, tc.secret)
			if !errors.Is(err, ErrCredentialControlByte) {
				t.Fatalf("NewCredential = %v, want ErrCredentialControlByte", err)
			}
			if !strings.Contains(err.Error(), tc.field) || !strings.Contains(err.Error(), tc.offset) {
				t.Fatalf("error %q omits field %q or offset %q", err, tc.field, tc.offset)
			}
			if strings.Contains(err.Error(), tc.value) {
				t.Fatalf("error %q leaks credential value", err)
			}
		})
	}
}

func TestSpendDomainValidationAndSignerAnswers(t *testing.T) {
	t.Parallel()
	for _, domain := range []SpendDomain{SpendDomainBinance, SpendDomainBybit} {
		if err := domain.Validate(); err != nil {
			t.Errorf("%q Validate: %v", domain, err)
		}
	}
	if err := SpendDomain("binance_prod").Validate(); !errors.Is(err, ErrSpendDomainInvalid) {
		t.Fatalf("undeclared domain Validate = %v, want ErrSpendDomainInvalid", err)
	}
	cred, err := NewCredential("key", "secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	binance, err := NewBinanceSigner("binance_prod", cred)
	if err != nil {
		t.Fatalf("NewBinanceSigner: %v", err)
	}
	bybit, err := NewBybitSigner("bybit_testnet", cred)
	if err != nil {
		t.Fatalf("NewBybitSigner: %v", err)
	}
	if binance.SpendDomain() != SpendDomainBinance || bybit.SpendDomain() != SpendDomainBybit {
		t.Fatalf("signer domains = (%q, %q)", binance.SpendDomain(), bybit.SpendDomain())
	}
	if binance.Fingerprint() != cred.Fingerprint() || bybit.Fingerprint() != cred.Fingerprint() {
		t.Fatal("signer fingerprint does not delegate to credential")
	}
}

func TestAcquireSpendLockKeysOnlyByDomain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	firstCred, _ := NewCredential("first-key", "secret")
	secondCred, _ := NewCredential("second-key", "secret")
	first, err := AcquireSpendLock(dir, SpendDomainBinance, firstCred)
	if err != nil {
		t.Fatalf("AcquireSpendLock first: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	if _, err := AcquireSpendLock(dir, SpendDomainBinance, secondCred); !errors.Is(err, ErrSpendLockHeld) {
		t.Fatalf("second credential same domain = %v, want ErrSpendLockHeld", err)
	}

	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(names) != 1 || names[0].Name() != "binance.lock" {
		t.Fatalf("lock names = %v, want [binance.lock]", names)
	}
	name := names[0].Name()
	for _, forbidden := range []string{"first-key", firstCred.Fingerprint(), "binance_prod"} {
		if strings.Contains(name, forbidden) {
			t.Fatalf("lock name %q contains %q", name, forbidden)
		}
	}
	if first.Domain() != SpendDomainBinance || first.Fingerprint() != firstCred.Fingerprint() {
		t.Fatalf("token metadata = (%q, %q)", first.Domain(), first.Fingerprint())
	}
	if first.path != filepath.Join(dir, "binance.lock") {
		t.Fatalf("token path = %q", first.path)
	}
}

func TestAcquireSpendLockCollidesAcrossVenueAliasesInOneDomain(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cred, err := NewCredential("shared-key", "shared-secret")
	if err != nil {
		t.Fatalf("NewCredential: %v", err)
	}
	prod, err := NewBinanceSigner("binance_prod", cred)
	if err != nil {
		t.Fatalf("NewBinanceSigner prod: %v", err)
	}
	testnet, err := NewBinanceSigner("binance_testnet", cred)
	if err != nil {
		t.Fatalf("NewBinanceSigner testnet: %v", err)
	}
	if prod.Venue() == testnet.Venue() {
		t.Fatalf("signer aliases collapsed to %q", prod.Venue())
	}
	if prod.SpendDomain() != SpendDomainBinance || testnet.SpendDomain() != SpendDomainBinance {
		t.Fatalf("signer domains = (%q, %q), want (%q, %q)",
			prod.SpendDomain(), testnet.SpendDomain(), SpendDomainBinance, SpendDomainBinance)
	}

	first, err := AcquireSpendLock(dir, prod.SpendDomain(), cred)
	if err != nil {
		t.Fatalf("AcquireSpendLock prod: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	second, err := AcquireSpendLock(dir, testnet.SpendDomain(), cred)
	if second != nil {
		_ = second.Close()
	}
	if !errors.Is(err, ErrSpendLockHeld) {
		t.Fatalf("AcquireSpendLock testnet alias = %v, want ErrSpendLockHeld", err)
	}
}

func TestSpendLockCloseReleasesAndIsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cred, _ := NewCredential("key", "secret")
	first, err := AcquireSpendLock(dir, SpendDomainBybit, cred)
	if err != nil {
		t.Fatalf("AcquireSpendLock: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	second, err := AcquireSpendLock(dir, SpendDomainBybit, cred)
	if err != nil {
		t.Fatalf("reacquire: %v", err)
	}
	_ = second.Close()
	if _, err := os.Stat(filepath.Join(dir, "bybit.lock")); err != nil {
		t.Fatalf("persistent lock file: %v", err)
	}
}

func TestAcquireSpendLockRefusesMissingInputs(t *testing.T) {
	t.Parallel()
	cred, _ := NewCredential("key", "secret")
	if _, err := AcquireSpendLock("", SpendDomainBinance, cred); !errors.Is(err, ErrSpendLockDirUnset) {
		t.Fatalf("empty dir = %v, want ErrSpendLockDirUnset", err)
	}
	if _, err := AcquireSpendLock(t.TempDir(), "", cred); !errors.Is(err, ErrSpendDomainInvalid) {
		t.Fatalf("empty domain = %v, want ErrSpendDomainInvalid", err)
	}
	if _, err := AcquireSpendLock(t.TempDir(), SpendDomainBinance, Credential{}); !errors.Is(err, ErrCredentialKeyUnset) {
		t.Fatalf("empty credential = %v, want ErrCredentialKeyUnset", err)
	}
}
