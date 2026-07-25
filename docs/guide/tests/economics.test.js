import { test, expect } from "bun:test";
import { loadPure } from "./extract.js";

// tutorial §3.9 worked trade: N=$10k, BTC flat at $100k, +0.015%/8h, 20-day hold, 3x perp,
// VIP0 taker, rescue margin ~$3,333, fee/slippage reserve ~$40.
const WORKED = {
  notional: 10000,
  perpLeverage: 3,
  rescueMarginUsd: 3333,
  feeReserveUsd: 40,
  committedFloatUsd: 0,
  fundingRatePerSettlement: 0.00015,
  fundingIntervalHours: 8,
  holdDays: 20,
  spotFeeRate: 0.0010,
  perpFeeRate: 0.0005,
  slippagePerLegRate: 0.0002,
  roundTripMultiplier: 1,
  utilization: 0.6,
  hurdleApy: 0.046
};

test("leg-capital floor is 1.33·N — tutorial §3.9, core §2.2", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.cLegs).toBeCloseTo(13333.33, 1);
});

test("C_allocated is about $16.7K — tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.cAllocated).toBeCloseTo(16706.33, 1);
});

test("60 settlements over a 20-day hold at 8h — tutorial §3.9", () => {
  expect(loadPure().computeEconomics(WORKED).settlements).toBe(60);
});

test("funding collected is $90 — tutorial §3.9", () => {
  expect(loadPure().computeEconomics(WORKED).fundingUsd).toBeCloseTo(90, 6);
});

test("round-trip cost is $38, i.e. 0.38% of notional — tutorial §3.9, core §2.4", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.roundTripCostUsd).toBeCloseTo(38, 6);
  expect(r.roundTripRate).toBeCloseTo(0.0038, 6);
});

test("net is +$52 over the hold — tutorial §3.9", () => {
  expect(loadPure().computeEconomics(WORKED).netUsd).toBeCloseTo(52, 6);
});

test("break-even takes about 25 settlements / 8.4 days — tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.breakEvenSettlements).toBeCloseTo(25.33, 1);
  expect(r.breakEvenDays).toBeCloseTo(8.44, 1);
});

test("in-position return is ~5.7% annualized — tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.inPositionApy * 100).toBeCloseTo(5.68, 1);
});

test("the C_legs denominator overstates it as ~7.1% — tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.apyOnLegsFloor * 100).toBeCloseTo(7.12, 1);
});

// THE NAMED TEST from the design doc, and the one core §12 predicts milestone 2 will hit.
test("at 60% utilization it nets ~3.4% and FAILS the 4.6% hurdle — core §12, tutorial §3.9", () => {
  const r = loadPure().computeEconomics(WORKED);
  expect(r.netApy * 100).toBeCloseTo(3.41, 1);
  expect(r.clearsHurdle).toBe(false);
  expect(r.shortfallPts).toBeCloseTo(-1.19, 1);
});

// core §2.4 break-even worked example: baseline +0.01%/8h against a 0.38% round trip.
test("baseline +0.01%/8h takes ~12-13 days to break even — core §2.4", () => {
  const r = loadPure().computeEconomics({ ...WORKED, fundingRatePerSettlement: 0.0001 });
  expect(r.breakEvenDays).toBeGreaterThan(12);
  expect(r.breakEvenDays).toBeLessThan(13);
});

test("a rich +0.02%/8h regime still takes ~6 days — core §2.4", () => {
  const r = loadPure().computeEconomics({ ...WORKED, fundingRatePerSettlement: 0.0002 });
  expect(r.breakEvenDays).toBeCloseTo(6.33, 1);
});

test("a 1h funding interval yields eight times the settlements — core C8", () => {
  const r = loadPure().computeEconomics({ ...WORKED, fundingIntervalHours: 1 });
  expect(r.settlements).toBe(480);
});

test("every preset is a complete, computable input", () => {
  const pure = loadPure();
  const keys = Object.keys(WORKED);
  for (const [id, preset] of Object.entries(pure.PRESETS)) {
    for (const k of keys) {
      expect(preset.input[k], `preset ${id} missing ${k}`).toBeDefined();
    }
    expect(Number.isFinite(pure.computeEconomics(preset.input).netApy)).toBe(true);
  }
});

test("the maker-entry fork halves round-trip cost and just clears the hurdle — core §2.4", () => {
  const pure = loadPure();
  const r = pure.computeEconomics(pure.PRESETS.maker.input);
  expect(r.roundTripCostUsd).toBeCloseTo(19, 6);
  expect(r.clearsHurdle).toBe(true);
  expect(r.netApy * 100).toBeCloseTo(4.65, 1);
});
