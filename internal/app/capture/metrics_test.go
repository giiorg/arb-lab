package capture

import (
	"testing"

	enginecapture "github.com/giiorg/arb-lab/internal/engine/capture"
	"github.com/giiorg/arb-lab/internal/platform/observability"
)

// TestEverySourcesLabelValuesAreDeclared: closedLabels folds label values over
// ALL sources, and every pipeline fixture runs exactly one — where "all
// sources" and "the first source" are the same set, the one-member collapse.
// Two sources with disjoint symbol and topic values, an observation recorded
// for the SECOND, and a rejection count of zero is §10's claim actually
// measured: the declared sets cover what the pipeline will record.
func TestEverySourcesLabelValuesAreDeclared(t *testing.T) {
	cfg := validConfig()
	second := validSource()
	second.Symbol = "ETHUSDT"
	second.Topic = "ethusdt@depth20@100ms"
	cfg.Sources = append(cfg.Sources, second)

	reg := observability.NewRegistry()
	m, err := newMetrics(reg, cfg)
	if err != nil {
		t.Fatalf("metrics over two sources: %v", err)
	}

	sm := newSourceMetrics(m, newFakeClock(), labelValues(cfg.Sources[1]))
	sm.countGap(enginecapture.CauseReconnect)
	sm.exportQueue(enginecapture.QueueStats{Drops: 1, HighWater: 2})
	sm.exportWriter(enginecapture.WriterStats{Records: 2, DataRecords: 1, Bytes: 64})

	if rej := reg.Rejected(); rej != 0 {
		t.Fatalf("the registry rejected %d observations for the second source; "+
			"its label values were not declared", rej)
	}
	found := false
	for _, s := range reg.Snapshot() {
		if s.Name != "capture_gaps_total" {
			continue
		}
		for _, v := range s.LabelValues {
			if v == "ETHUSDT" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no capture_gaps_total series carries the second source's symbol")
	}
}
