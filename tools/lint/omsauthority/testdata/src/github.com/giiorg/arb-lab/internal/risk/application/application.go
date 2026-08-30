// Package application is a bounded context taking OMS authority, with its clean
// imports beside its offending ones so the diagnostics are shown to land on the
// right lines rather than merely to exist.
package application

import (
	tp "example.com/thirdparty/oms/application"

	"github.com/giiorg/arb-lab/internal/oms"                    // want `not granted OMS write authority.*OMS control surface`
	omsapp "github.com/giiorg/arb-lab/internal/oms/application" // want `not granted OMS write authority.*the use cases themselves`
	"github.com/giiorg/arb-lab/internal/oms/application/ports"
	"github.com/giiorg/arb-lab/internal/oms/application/usecases" // want `not granted OMS write authority.*the use cases themselves`
	"github.com/giiorg/arb-lab/internal/oms/applicationx"
	"github.com/giiorg/arb-lab/internal/oms/domain"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/baseline"
	"github.com/giiorg/arb-lab/internal/oms/infrastructure/driven"
)

// Held names one thing from each import, so none is dropped as unused and every
// line above is a real edge.
type Held struct {
	Third    tp.Executor
	Module   oms.Module
	Executor omsapp.Executor
	Port     ports.OrderWriteRepository
	Ladder   usecases.Delever
	NotOne   applicationx.NotAUseCase
	State    domain.OrderState
	Pin      baseline.Pin
	Repo     driven.Repo
}
