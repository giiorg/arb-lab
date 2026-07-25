# Translation glossary — English → Georgian

Frozen before bulk translation. Passed verbatim in every prompt. Never edited
mid-run: terminology drift between sections is the failure mode this prevents.

## Keep in Latin script, untranslated

Spec and venue vocabulary — what exchange UIs show and what the normative docs
call these things:

perpetual, perp, spot, futures, funding rate, funding interval, settlement,
mark price, index price, last price, basis, maker, taker, slippage, notional,
APY, hurdle, round trip, utilization, ADL, auto-deleveraging, fence, actuator,
order book, VIP0, Binance, Bybit, BTC, USDT, USDC, bracket, IOC

**leg** — always Latin, never translated. A *leg* is one side of the two-sided
position (the `spot leg`, the `perp leg`), which is what `C_legs`, "one-leg
failure" and "two-leg close" all refer to. Do NOT render it as "ეტაპი" (stage),
"ნაწილი" (part) or "ფეხი" — those lose the meaning the whole strategy rests on.
Write `spot leg` and `perp leg` exactly so.

**spread**, **fill**, **open interest** — Latin, same reason.

## Translate into Georgian

| English | Georgian |
|---|---|
| exchange (the venue) | ბირჟა |
| fee | საკომისიო |
| profit / loss | მოგება / ზარალი |
| price | ფასი |
| position | პოზიცია |
| account | ანგარიში |
| capital / bankroll | კაპიტალი |
| risk | რისკი |
| cost | ხარჯი |
| trade (noun) | გარიგება |
| margin | მარჟა |
| liquidation | ლიკვიდაცია |
| leverage | ბერკეტი |
| hedge | ჰეჯი (verb: ჰეჯირება) |
| break-even | უზარალობა |
| collateral | უზრუნველყოფა |

## Never touched in either language

Citation chip contents — `core §7.4`, `tutorial §3.9`, `Profiles §1`, `SM §4` —
are pointers into English normative documents. Neither translated nor
transliterated. A test enforces this.

Numbers, currency and percentages keep their English formatting (`$10,000`,
`3.41%`), because they must stay tied to the figures printed in the spec.
