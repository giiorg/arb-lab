# Record §91 — task #234: the freshness gate demanded a value it refused to print, and the message could not be tested because it had no name

**Status:** landed, 2026-08-09. Milestone 4, task **#234**, a defect in record
**§88**'s tooling rather than in any model or spec section. **Code only** —
`tools/formalfresh/advice.go` (new), `advice_test.go` (new),
`freshness_test.go` (rewired). No normative document changed, so no doc version
moves and no digest is re-pinned.

---

## 1. The defect

`TestManifestSubjectsAreWellFormed` refuses any `knownStale` marker whose
`sha256Actual` is not exactly 64 characters:

```go
if len(s.KnownStale.SHA256) != 64 {
    t.Errorf("%s: %s marker has a malformed sha256Actual %q", ...)
}
```

The `Stale` verdict — the one whose closing sentence is *"or record a knownStale
marker naming the task that will"* — printed `short(actual)`, **12 characters**.
So the gate stated a requirement and, in the same breath, withheld the only value
that satisfies it. The reader's next move was to go and recompute a digest the
gate had held in a local variable one line earlier.

`MarkerStale` had it too, and identically: it says *"update the marker"*, and
updating the marker means writing the current digest into `sha256Actual`. Two
sites, one defect — the mirror rule applied on the first reading rather than
after a survivor.

It was found by **being the gate's first re-pinner** (session 134, #233). Nothing
about the code changed to expose it; §88 landed with zero markers recorded, so
until a real staleness had to be marked, the message was prose nobody had
executed against.

## 2. Why no test could have caught it

The messages were format strings inside `t.Errorf` calls in `freshness_test.go`.
That has two consequences and the second is the worse one:

- **Nothing could assert on them.** A message that lives only as an argument to
  `t.Errorf` in a test function is unreachable from any other test.
- **`tools/mutate.sh` had no line to aim at.** A bite-check needs a line whose
  breaking makes some suite fail; a line inside the only suite that would report
  it is not that line.

This is the repo's own **"a claim that cannot be mutated wants a name"**, and the
fix therefore starts by extracting rather than by editing. `Explain(verdict,
model, subject, actual) Report` renders every verdict; the gate keeps only the
decision to fail the build or merely narrate:

```go
switch r := Explain(s.Check(actual), model, s, actual); {
case r.Fails:      t.Error(r.Text)
case r.Text != "": t.Log(r.Text)
}
```

`Report{Text, Fails}` exists so that the *severity* of a verdict is data too. It
was previously encoded in which `testing.T` method the arm happened to call,
which is another claim with no name.

## 3. The rule the fix establishes

**A digest the reader must TRANSCRIBE is printed in full; a digest the reader
only COMPARES stays truncated.**

The asymmetry is deliberate and is recorded because it will otherwise read as an
oversight in a future review. `stamped:` and `marker describes:` remain 12
characters: their full values are already in `docs/formal/freshness.json`, which
is the file the reader is about to open anyway, and their job in the message is
to let a person see *that* something changed. Printing all four digests whole
would put four 64-character lines in one failure and bury the single line that
has to be copied.

The restamp path is the third case and needs neither, because it rewrites the
manifest itself — nothing is transcribed there, so its log line stays short on
both sides.

## 4. How it was checked

**Unit, 8 mutants / 0 survivors** (`tools/mutate.sh`, target
`./tools/formalfresh`): both truncation sites re-introduced (the defect itself
and its mirror), both **argument swaps**, `Fails` flipped on `Stale`, `Fresh`
made to fail, `short` truncating to 11, and `short` dropping an undersized
digest.

Two fixture decisions did real work, and both are collapse-checklist items:

- **The fixture digests are 64 characters.** At 12 or fewer, `short(d) == d`, so
  "printed in full" and "printed truncated" are the *same string* and every
  assertion passes against the very defect #234 names. The test `panic`s in
  `init()` if that premise ever stops holding, because a fixture that quietly
  stopped satisfying its own precondition would take the assertions down without
  failing anything.
- **The digests are asserted POSITIONALLY, by line label.** The first draft
  asserted only `strings.Contains(text, full)`. That is satisfied by swapping the
  two arguments — the full digest then prints under `stamped:` and the reader is
  told to copy the digest they already had. Both slots hold a value of identical
  width and character class, which is the *format-collapse* shape exactly. The
  swap mutants above are what confirm the positional assertions bite; against the
  first draft they would have SURVIVED.

**End-to-end, on the live manifest.** Unit tests establish that `Explain` is
right, not that the gate uses it, and not that the digest it prints is the
*correct* one. So: a line was inserted into `build-spec.md` §7.4 (a premise
tracked by **both** models), `make formal-fresh` was run, the 64-character digest
was copied **verbatim** out of the failure text into a `knownStale` marker on
both models' subjects, and the gate was re-run. It went green with both sections
reporting `marked stale`. Then both files were reverted and the gate returned to
baseline.

That loop is the actual user-facing claim of #234 — *the value in the message is
the value the manifest wants* — and no unit test reaches it. A message can print
a full-length digest of the wrong thing and satisfy every assertion in §4's first
paragraph.

## 5. Rejected alternatives

**Print a paste-ready `knownStale` JSON block in the failure.** More convenient,
and rejected: it would state the manifest's schema a second time, in a format
string, where nothing type-checks it against `KnownStale`. A message that drifts
from the struct it claims to describe is a worse failure than a message that
makes you type two fields, and this repo has a memory for justifications that
outlive their premise. Naming the field (`sha256Actual`) in prose gets most of
the benefit at none of the cost.

**Print every digest in full.** Rejected on the reading of the message: the
failure's purpose is to route a person to one of two actions, and four
64-character lines make the discriminating line harder to find, not easier. See
§3.

**Widen `short()` to 64 and leave the call sites alone.** This is the same as the
above with the asymmetry deleted, and it also removes the only place the rule in
§3 is expressed.

**Fix the two format strings in place and skip the extraction.** Rejected: it
lands the correct bytes and leaves the claim untestable and unmutatable, so the
next edit to either message has exactly the protection this one had — none. The
extraction is the durable half of the task; the digit count is the symptom.

## 6. What this does not do

`short()` is still applied to the restamp log line on both sides (§3). It is
correct there and it is the one place where a future reader could reasonably ask
"why not this one too", which is why it is named here rather than left to be
re-derived.

Nothing in this record touches what the gate *tracks*. #231 and #232 remain the
two open findings from §88's first run, and #229 remains the model revision that
owns the two `knownStale` markers on `state-machines.md` §2 and §7.
