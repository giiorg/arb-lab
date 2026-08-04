package transport

import "fmt"

// SpendDomain is the closed, compiled name of an exchange budget. The lock key
// cannot be a credential because venues also meter per IP or UID across keys,
// and it cannot be the open venue alias because two aliases could name one
// exchange and take different files. A new dialect must therefore add a member
// here rather than choose a namespace at runtime.
type SpendDomain string

const (
	SpendDomainBinance SpendDomain = "binance"
	SpendDomainBybit   SpendDomain = "bybit"
)

// Validate reports whether the domain is one of the compiled filesystem-safe
// members. The zero value and invented aliases are unusable so the lock cannot
// silently guard a namespace no signer shares.
func (d SpendDomain) Validate() error {
	switch d {
	case SpendDomainBinance, SpendDomainBybit:
		return nil
	case "":
		return fmt.Errorf("spend domain is empty: %w", ErrSpendDomainInvalid)
	default:
		return fmt.Errorf("spend domain %q is undeclared: %w", d, ErrSpendDomainInvalid)
	}
}
