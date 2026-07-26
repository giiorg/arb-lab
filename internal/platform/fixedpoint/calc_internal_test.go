package fixedpoint

import (
	"errors"
	"testing"
)

// Result must discard the running value whenever there is an error, and this
// has to be tested from inside the package.
//
// From outside it is unreachable: every operation returns Money{} alongside its
// error, so a failed chain's running value is already invalid and `return
// c.money, err` would behave identically. That makes the guard in Result look
// redundant — and it is, today. It stops being redundant the moment someone
// adds an operation that returns a partial value with an error, which is a
// natural thing to write and would then leak a spendable amount out of a failed
// calculation.
//
// Constructing the state directly is the only way to state the contract
// independently of what the current operations happen to do.
func TestCalc_ResultDiscardsTheValueOnError(t *testing.T) {
	valid, err := ParseMoney("USDT", "100")
	if err != nil {
		t.Fatalf("ParseMoney failed: %v", err)
	}
	sentinel := errors.New("a step failed but left a value behind")

	leaky := Calc{money: valid, err: sentinel}

	got, gotErr := leaky.Result()
	if !errors.Is(gotErr, sentinel) {
		t.Fatalf("Result error = %v; want the recorded error", gotErr)
	}
	if got.Validate() == nil {
		t.Errorf("Result returned the valid %s alongside an error; "+
			"a failed calculation must not yield a usable amount", got)
	}

	rate, gotErr := leaky.Ratio(valid, RoundExact)
	if !errors.Is(gotErr, sentinel) {
		t.Fatalf("Ratio error = %v; want the recorded error", gotErr)
	}
	if rate.Validate() == nil {
		t.Errorf("Ratio returned a constructed Rate alongside an error")
	}
}
