@/home/ted/projects/share/tandem-protocol/tandem-protocol.md

# Software Development Simulation

## Reference Implementation Principles

This project is a **reference implementation for scaling**. Always choose the architecturally correct approach over expedient solutions.

### Architectural Decision Framework

When multiple solutions exist, evaluate against these guides (in `urma-obsidian/guides/`):

| Guide | Key Principle | Example Violation |
|-------|---------------|-------------------|
| **ES Guide** | Events are source of truth, projections derived | Caching projection in mutable field |
| **FP Guide §7** | Immutability - no shared mutable state | `eng.proj = newProj` races with readers |
| **Go Dev Guide §3** | Value semantics - return values, not mutate | Pointer receivers that mutate state |

### Concurrency: Immutable Engine Pattern

**Required approach**: Operations return new Engine, no mutable cached state.

```go
// CORRECT: Immutable - operation returns new Engine
func (e Engine) Tick() Engine {
    newProj := e.store.Replay().Apply(newEvents...)
    return Engine{store: e.store, proj: newProj}
}
// Usage: eng = eng.Tick()

// WRONG: Mutable field races between emit() and Sim()
func (e *Engine) emit(event Event) {
    e.proj = e.proj.Apply(event)  // RACE with Sim() readers
}
```

**Why not RWMutex?** While `sync.RWMutex` works (tested), it:
- Adds contention on high-read workloads
- Violates FP Guide §7 (still has mutable state, just protected)
- Doesn't align with ES philosophy (projection derived on demand)

**Why not atomic pointer?** `atomic.Pointer[Projection]` is lock-free but still mutates the Engine's internal state - same FP violation.

### Trade-off Acknowledgment

The immutable pattern has costs:
- More allocations (new Engine per operation)
- Verbose call sites (`eng = eng.Tick()` vs `eng.Tick()`)
- May need optimization for very high-frequency operations

These are acceptable for a reference implementation that prioritizes correctness.

## Development Environment

- **Language**: Go
- **Package Management**: Nix flakes for ongoing development
- **Ephemeral Tools**: nix-shell for one-off tool needs

## Code Style: FluentFP

Use `github.com/binaryphile/fluentfp` for fluent, functional patterns where they afford concise but clear code.

### slice Package - Complete API

```go
import "github.com/binaryphile/fluentfp/slice"

// Factory functions
slice.From(ts []T) Mapper[T]           // For mapping to built-in types
slice.MapTo[R](ts []T) MapperTo[R,T]   // For mapping to arbitrary type R

// Mapper[T] methods (also on MapperTo)
.KeepIf(fn func(T) bool) Mapper[T]     // Filter: keep matching
.RemoveIf(fn func(T) bool) Mapper[T]   // Filter: remove matching
.Convert(fn func(T) T) Mapper[T]       // Map to same type
.TakeFirst(n int) Mapper[T]            // First n elements
.Each(fn func(T))                      // Side-effect iteration
.Len() int                             // Count elements

// Mapping methods (return Mapper of target type)
.ToAny(fn func(T) any) Mapper[any]
.ToBool(fn func(T) bool) Mapper[bool]
.ToByte(fn func(T) byte) Mapper[byte]
.ToError(fn func(T) error) Mapper[error]
.ToFloat32(fn func(T) float32) Mapper[float32]
.ToFloat64(fn func(T) float64) Mapper[float64]
.ToInt(fn func(T) int) Mapper[int]
.ToRune(fn func(T) rune) Mapper[rune]
.ToString(fn func(T) string) Mapper[string]

// MapperTo[R,T] additional method
.To(fn func(T) R) Mapper[R]            // Map to type R
```

### slice Patterns

```go
// Count matching elements (2 operations = multiline)
count := slice.From(tickets).
    KeepIf(Ticket.IsActive).
    Len()

// Extract field to strings (1 operation = single line)
ids := slice.From(tickets).ToString(Ticket.GetID)

// Method expressions for clean chains
actives := slice.From(users).
    Convert(User.Normalize).
    KeepIf(User.IsValid)
```

### Chain Formatting

**Single operation** - one line:
```go
names := slice.From(users).ToString(User.Name)
```

**Two+ operations** - each on indented line, trailing dot:
```go
count := slice.From(tickets).
    KeepIf(completedAfterCutoff).
    Len()
```

Setup (`slice.From`, `slice.MapTo[R]`) doesn't count—only chained methods count. This applies to all fluent-style calls.

### option Package

```go
import "github.com/binaryphile/fluentfp/option"

// Creating options
option.Of(t T) Basic[T]                // Always ok
option.New(t T, ok bool) Basic[T]      // Conditional ok
option.IfNotZero(t T) Basic[T]         // Ok if non-zero value (comparable types)
option.IfNotNil(ptr *T) Basic[T]       // From pointer (nil = not-ok)

// Using options
.Get() (T, bool)                       // Comma-ok unwrap
.Or(t T) T                             // Value or default
.OrZero() T                            // Value or zero
.OrEmpty() T                           // Alias for strings
.OrFalse() bool                        // For option.Bool
.Call(fn func(T))                      // Side-effect if ok

// Pre-defined types
option.String, option.Int, option.Bool, option.Error
```

### option Patterns

```go
// Nullable database field
func (r Record) GetHost() option.String {
    return option.IfNotZero(r.NullableHost.String)
}

// Tri-state boolean (true/false/unknown)
type Result struct {
    IsConnected option.Bool  // OrFalse() gives default
}
connected := result.IsConnected.OrFalse()
```

### Pseudo-Option Conventions

Pseudo-options use Go's zero values to represent "absent" without formal Option types.

| Style | Convention | Detection | Conversion to Option |
|-------|------------|-----------|---------------------|
| Pointer (`*T`) | Suffix variable with `Opt` | `ptr != nil` | `option.IfNotNil(ptr)` |
| Zero-value (comparable) | No suffix needed | `t != zero` | `option.IfNotZero(t)` |

**Pointer pseudo-options (`*T`):**
```go
configOpt *Config  // Suffix with "Opt" to clarify nil=absent semantics
if configOpt != nil { ... }
```

**Zero-value pseudo-options (non-comparable structs):**
```go
// Add IsZero method to enable zero-value-as-absent pattern
type Registry struct {
    instances map[string]Instance
}

func (r Registry) IsZero() bool { return r.instances == nil }

// Usage: pass zero value for "none"
func NewApp(registry Registry) *App {
    if !registry.IsZero() {
        // use registry
    }
}
app := NewApp(Registry{})  // standalone mode (no registry)
```

### Prefer Options Over Nil Pointers

For optional values, prefer `option.Basic[T]` over `*T`:

| Pattern | Problem | Solution |
|---------|---------|----------|
| `*T` for "maybe absent" | Nil checks scattered, panic risk | `option.Basic[T]` |
| `if ptr != nil` | Easy to forget, silent bugs | `.Get()` forces handling |
| `*ptr` access | Panics if nil | `.OrZero()` or `.Or(default)` |

**Use pointers for:** `sync.Mutex` fields, interface requirements, external handles, or profiled hot paths >10MB.
**Use options for:** "This value may not exist" semantics.

### either Package

Either represents a value that is one of two possible types (Left or Right). Convention: Left = failure/alternative, Right = success/primary.

```go
import "github.com/binaryphile/fluentfp/either"

// Creating Either values
either.Left[L, R](l L) Either[L, R]   // Create Left variant
either.Right[L, R](r R) Either[L, R]  // Create Right variant

// Accessing values (comma-ok pattern)
.Get() (R, bool)                      // Get Right value
.GetLeft() (L, bool)                  // Get Left value
.IsLeft() bool                        // Check if Left
.IsRight() bool                       // Check if Right
.MustGet() R                          // Right value or panic
.MustGetLeft() L                      // Left value or panic

// Defaults
.GetOr(default R) R                   // Right value or default
.LeftOr(default L) L                  // Left value or default
.GetOrCall(fn func() R) R             // Right value or lazy default
.LeftOrCall(fn func() L) L            // Left value or lazy default

// Transforming
either.Fold(e, leftFn, rightFn) T     // Exhaustive pattern match
either.Map(e, fn func(R) R2) Either[L, R2]    // Transform Right (type-changing)
either.MapLeft(e, fn func(L) L2) Either[L2, R] // Transform Left (type-changing)
.Map(fn func(R) R) Either[L, R]       // Transform Right (same type)

// Side effects
.Call(fn func(R))                     // Execute fn if Right
.CallLeft(fn func(L))                 // Execute fn if Left
```

### either Patterns

**Mutually exclusive modes** - when a struct can be in one of two states:

```go
// BAD: Two nullable fields with scattered nil checks
type App struct {
    client *Client        // nil when engine mode
    engine *engine.Engine // nil when client mode
}
if a.client != nil { /* client mode */ }

// GOOD: Explicit Either with grouped mode state
type ClientMode struct {
    Client *Client
    SimID  string
}

type EngineMode struct {
    Engine  *engine.Engine
    Tracker metrics.Tracker
}

type App struct {
    mode either.Either[EngineMode, ClientMode]
}

// Access with Fold for exhaustive handling
simID := either.Fold(a.mode,
    func(eng EngineMode) string { return eng.Engine.Sim().ID },
    func(cli ClientMode) string { return cli.SimID },
)
```

**Operation that can fail with reason** - when you need more than just success/failure:

```go
// BAD: Boolean gives no context on why
func (e *Engine) TryDecompose(id string) ([]Ticket, bool)

// GOOD: Either provides failure reason
type NotDecomposable struct {
    Reason string // "not found", "policy forbids", etc.
}

func (e *Engine) TryDecompose(id string) either.Either[NotDecomposable, []Ticket]

// Caller can handle with context
result := engine.TryDecompose("TKT-001")
if tickets, ok := result.Get(); ok {
    // Use tickets
} else if notDecomp, ok := result.GetLeft(); ok {
    log.Printf("Cannot decompose: %s", notDecomp.Reason)
}
```

### Either vs Option

| Pattern | Use | Example |
|---------|-----|---------|
| `option.Basic[T]` | Value may be absent | Database nullable field |
| `either.Either[L, R]` | One of two distinct states | Mode A or Mode B |

Option is for "maybe nothing." Either is for "definitely something, but which one?"

### must Package

```go
import "github.com/binaryphile/fluentfp/must"

must.Get(t T, err error) T                    // Return or panic
must.Get2(t T, t2 T2, err error) (T, T2)      // 3-return variant
must.BeNil(err error)                         // Panic if error
must.Getenv(key string) string                // Env var or panic
must.Of(fn func(T) (R, error)) func(T) R      // Wrap fallible func
```

### must Patterns

```go
// With slice operations (prefix with "must" to signal panic behavior)
mustAtoi := must.Of(strconv.Atoi)
ints := slice.From(strings).ToInt(mustAtoi)

// Inline in expressions
devices = append(devices, must.Get(store.GetDevices(chunk))...)

// Initialization sequences
db := must.Get(sql.Open("postgres", dsn))
must.BeNil(db.Ping())

// Time parsing
timestamp := must.Get(time.Parse("2006-01-02 15:04:05", s.ScannedAt))

// Invariants: errors that should never happen at runtime
// Use must when the error represents a bug, not a runtime condition
eng = must.Get(eng.StartSprint())           // Single-user: no concurrent conflicts
eng, events = must.Get2(eng.Tick())         // 3-return: (Engine, []Event, error)
sprint := sim.CurrentSprintOption.MustGet() // Option has MustGet() for invariants
```

**When to use must vs error handling:**
- `must.Get`: Error indicates a bug (invariant violation), not a runtime condition
- Error handling: Error is expected and recoverable (user input, network, file I/O)
- If you think "this error can never happen here", use `must` to enforce that invariant

### ternary Package

```go
import "github.com/binaryphile/fluentfp/ternary"

ternary.If[R](cond bool).Then(t R).Else(e R) R
ternary.If[R](cond bool).ThenCall(fn).ElseCall(fn) R  // Lazy
```

### ternary Patterns

```go
// Factory alias for repeated use
If := ternary.If[string]
status := If(done).Then("complete").Else("pending")
```

### lof Package (Lower-Order Functions)

```go
import "github.com/binaryphile/fluentfp/lof"

lof.Println(s string)      // Wraps fmt.Println for Each
lof.Len(ts []T) int        // Wraps len
```

### pair Package (Tuples)

```go
import "github.com/binaryphile/fluentfp/tuple/pair"

// Pair type
pair.X[V1, V2]             // Struct with V1, V2 fields

// Creating pairs
pair.Of(v1, v2) X[V1,V2]   // Construct a pair

// Zipping slices
pair.Zip(as, bs) []X[A,B]           // Combine into pairs (panics if unequal length)
pair.ZipWith(as, bs, fn) []R        // Combine and transform (panics if unequal length)
```

### pair Patterns

```go
// Parallel slice iteration
pairs := pair.Zip(names, ages)
for _, p := range pairs {
    fmt.Printf("%s is %d\n", p.V1, p.V2)
}

// Direct transformation without intermediate pairs
users := pair.ZipWith(names, ages, NewUserFromNameAge)

// Chain with slice.From for filtering
adults := slice.From(pair.Zip(names, ages)).KeepIf(NameAgePairIsAdult)
```

### Fold and Unzip (v0.6.0)

```go
// Fold - reduce slice to single value
// sumFloat64 adds two float64 values.
sumFloat64 := func(acc, x float64) float64 { return acc + x }
total := slice.Fold(amounts, 0.0, sumFloat64)

// Build map from slice
// indexByMAC adds a device to the map keyed by its MAC address.
indexByMAC := func(m map[string]Device, d Device) map[string]Device {
    m[d.MAC] = d
    return m
}
byMAC := slice.Fold(devices, make(map[string]Device), indexByMAC)

// Unzip - extract multiple fields in one pass (avoids N iterations)
// Use method expressions when types have appropriate getters
leadTimes, deployFreqs, mttrs, cfrs := slice.Unzip4(history,
    HistoryPoint.GetLeadTimeAvg,
    HistoryPoint.GetDeployFrequency,
    HistoryPoint.GetMTTR,
    HistoryPoint.GetChangeFailRate,
)
```

### Named Functions in FluentFP Chains

**This guidance applies to FluentFP chains only.** For simple if statements, inline conditions are clearer:

```go
// Simple if: inline is fine
if ticket.EstimatedDays > 5 {
    failRate *= 1.5
}

// DON'T extract predicates for simple ifs - adds ceremony without benefit
isLargeTicket := func(t Ticket) bool { return t.EstimatedDays > 5 }
if isLargeTicket(ticket) { ... }  // Overkill
```

**For FluentFP chains**, prefer named functions over inline lambdas:

```go
// GOOD: Named predicate with leading comment
// completedAfterCutoff returns true if ticket was completed after the cutoff tick.
completedAfterCutoff := func(t Ticket) bool { return t.CompletedTick >= cutoff }
count := slice.From(tickets).KeepIf(completedAfterCutoff).Len()

// BAD: Inline lambda in chain - harder to read
count := slice.From(tickets).KeepIf(func(t Ticket) bool { return t.CompletedTick >= cutoff }).Len()
```

**Preference hierarchy for FluentFP:**
1. **Method expressions** - `User.IsActive`, `Developer.IsIdle` (cleanest)
2. **Named functions** - `completedAfterCutoff` (readable, documented)
3. **Inline lambdas** - fallback when trivial and one-time

**Decision flowchart:**
```
Method on type? → YES: Use method expression
                → NO: Domain meaning? → YES: Named function with comment
                                      → NO: Trivial? → YES: Inline ok
                                                     → NO: Name it
```

See FP Guide Section 13 for point-free style, partial application, and pipeline formatting theory.

**All named functions get leading comments:**

```go
// completedAfterCutoff returns true if ticket was completed after the cutoff tick.
completedAfterCutoff := func(t Ticket) bool { return t.CompletedTick >= cutoff }
```

**Single-expression functions go on one line:**

```go
// sumDuration adds two durations.
sumDuration := func(a, b time.Duration) time.Duration { return a + b }

// GetPercentUsed returns the buffer consumption percentage.
func (s FeverSnapshot) GetPercentUsed() float64 { return s.PercentUsed }
```

**Why this matters:**

Anonymous functions require parsing lambda syntax, predicate logic, and chain context simultaneously. Named functions with leading comments let you read intent directly:

```go
// Inline: parse syntax, logic, and context together
slice.From(tickets).KeepIf(func(t Ticket) bool { return t.CompletedTick >= cutoff }).Len()

// Named: read intent - "keep if completed after cutoff"
slice.From(tickets).KeepIf(completedAfterCutoff).Len()
```

**Locality:** Define named functions close to first usage, not at package level.

#### Method Expressions (preferred)

When a type has a method matching the required signature, use it directly:
```go
// Method expression: reads as English, no function body to parse
slice.From(developers).KeepIf(Developer.IsIdle)

// Inline anonymous: reader must parse lambda syntax
slice.From(developers).KeepIf(func(d Developer) bool { return d.IsIdle() })
```

The method expression reads as intent: "keep if developer is idle." No syntax to parse, no function body—just *what*, not *how*.

**Critical: Use value receivers for read-only methods.** Method expressions only work when receiver type matches slice element type. `slice.From(users)` creates `Mapper[User]`, so `User.Method` requires a value receiver:

```go
// Works with slice.From
func (u User) IsActive() bool { return u.Active }

// Doesn't work - (*User).IsActive expects *User, not User
func (u *User) IsActive() bool { return u.Active }
```

**Design rule:** Value receivers by default, pointer receivers only when mutating. This:
- Enables method expressions with FluentFP
- Eliminates nil receiver panics (the "billion dollar mistake")
- Makes value semantics explicit

**Adding accessor methods for FluentFP:** When a type lacks methods for field extraction (common with data types), add accessor methods to enable method expressions:

```go
// DORASnapshot is a pure data type - add accessors for FluentFP
type DORASnapshot struct {
    LeadTimeAvg     float64
    DeployFrequency float64
}

// Accessors for FluentFP method expression syntax
func (s DORASnapshot) GetLeadTimeAvg() float64     { return s.LeadTimeAvg }
func (s DORASnapshot) GetDeployFrequency() float64 { return s.DeployFrequency }

// Now Unzip4 uses method expressions instead of lambdas
leadTimes, freqs := slice.Unzip2(history,
    DORASnapshot.GetLeadTimeAvg,
    DORASnapshot.GetDeployFrequency,
)
```

**When to add accessors:** Add `Get[FieldName]` methods when the type is used in FluentFP operations (ToFloat64, Unzip, etc.) more than once. Don't add accessors speculatively.

#### Named Functions (when method expressions don't apply)

When you need custom logic or the type lacks an appropriate method. **Single-expression predicates go on one line:**
```go
// completedAfterCutoff returns true if ticket was completed after the cutoff tick.
completedAfterCutoff := func(t Ticket) bool { return t.CompletedTick >= cutoff }
count := slice.From(tickets).KeepIf(completedAfterCutoff).Len()
```

For multi-statement bodies, use standard formatting:
```go
// complexCheck performs multiple validations.
complexCheck := func(u User) bool {
    if u.IsDeleted() {
        return false
    }
    return u.IsActive() && u.HasPermission("read")
}
```

#### Predicate Naming Patterns

| Pattern | When to use | Example |
|---------|-------------|---------|
| `Is[Condition]` | Simple check, subject obvious | `IsValidMAC` |
| `[Subject]Is[Condition]` | State check on specific type | `SliceOfScansIsEmpty` |
| `[Subject]Has[Condition](params)` | Parameterized predicate factory | `DeviceHasHWVersion("EX12")` |
| `Type.Is[Condition]` | Method expression | `Device.IsActive` |

#### Reducer Naming

```go
// sumFloat64 adds two float64 values.
sumFloat64 := func(acc, x float64) float64 { return acc + x }
total := slice.Fold(amounts, 0.0, sumFloat64)
```

### Why Prefer FluentFP for Data Transformation

**Concrete example - field extraction:**

```go
// FluentFP: extract percent values from history
return slice.From(f.History).ToFloat64(FeverSnapshot.GetPercentUsed)

// Loop: four concepts interleaved
// Extract percent values from history
var result []float64                           // 1. variable declaration
for _, s := range f.History {                  // 2. iteration mechanics (discarded _)
    result = append(result, s.PercentUsed)     // 3. append mechanics
}
return result                                  // 4. return
```

The loop forces you to think about *how* (declare, iterate, append, return). FluentFP expresses *what* (extract PercentUsed).

**General principles:**
- Loops have multiple forms → mental load
- Loops force wasted syntax (discarded `_` values)
- Loops nest; FluentFP chains
- Loops describe *how*; FluentFP describes *what*

**Performance note:** Mapping operations (ToFloat64, ToString) match or beat loops. Filter operations (KeepIf) allocate intermediate slices—our benchmarks show 4-9× overhead depending on pattern. Use loops for filter+count in hot paths. See Benchmarks section below for measured results.

### When Loops Are Still Necessary

1. **Channel consumption** - `for r := range chan` has no FP equivalent
2. **Complex control flow** - break/continue/early return within loop
3. **Index-dependent logic** - when you need `i` for more than just indexing

See [fluentfp/slice/README.md](https://github.com/binaryphile/fluentfp/blob/develop/slice/README.md#when-loops-are-still-necessary) for detailed examples.

### FluentFP Enhancements Wanted

- [x] Add `ToFloat64` and `ToFloat32` methods to slice package (v0.5.0)
- [x] Add `Fold`/`Reduce` for accumulating operations (v0.6.0)
- [x] Add `Unzip2`/`Unzip3`/`Unzip4` for multi-field extraction (v0.6.0)
- [x] Add `Zip`/`ZipWith` for parallel slice iteration (v0.6.0)
- [x] Add `ToInt32`/`ToInt64` methods to slice package (v0.8.0)
- [x] Add `either` package for sum types (v0.8.0)

## Value Semantics

### The Case for Values Over Pointers

Go's default to pass-by-value has underappreciated benefits. Consider preferring value semantics wherever practical, reserving pointers for designs that get dramatic simplification from them.

**Benefits of value semantics:**

1. **Nil safety** - Value receivers can't panic on nil. This eliminates an entire class of runtime errors (Hoare's "billion dollar mistake").

2. **Explicit mutation** - When methods return new values instead of mutating, call sites show what changes:
   ```go
   // Value semantics: mutation is visible
   sprint = sprint.WithConsumedBuffer(0.5)

   // Pointer semantics: mutation is hidden
   sprint.ConsumeBuffer(0.5)  // Did sprint change? Must read the method.
   ```

3. **No indirection** - `dev.IsIdle()` reads cleaner than `(*dev).IsIdle()` or the implicit indirection of pointer receivers.

4. **Method expressions** - FluentFP's `slice.From(users).KeepIf(User.IsActive)` requires value receivers. Pointer receivers break method expression compatibility.

5. **Predictable copying** - Small structs copy cheaply. Worrying about "copying overhead" is often premature optimization.

**The `With*` pattern for transformation:**

Instead of mutating methods, return transformed copies:
```go
// Value semantics
func (d Developer) WithTicket(id string) Developer {
    d.CurrentTicket = id
    d.WIPCount++
    return d
}
// Usage: dev = dev.WithTicket("TKT-001")

// vs pointer mutation
func (d *Developer) Assign(id string) {
    d.CurrentTicket = id
    d.WIPCount++
}
// Usage: dev.Assign("TKT-001")
```

The `With*` pattern makes mutation explicit at call sites. The trade-off is slightly more verbose call sites (`dev = dev.WithTicket(...)` vs `dev.WithTicket(...)`), but the explicitness aids comprehension.

**When structs REQUIRE pointer semantics:**

- **`sync.Mutex` fields** - Must not be copied after first use. This is the primary case.
- **Other sync primitives** - `sync.WaitGroup`, `sync.Cond`, `sync.Once`, etc.

**When pointers are a choice, not a requirement:**

- **Interface satisfaction** - When an interface requires pointer receivers.
- **Profiled hot paths >10MB** - Only after benchmarking proves copying is expensive.

**Containing pointers doesn't require pointer semantics:** A struct containing `*http.Client` or `*rand.Rand` can still use value receivers. When the struct is copied, both copies share the same underlying pointer. Example:
```go
// Client is a value type - copying shares the *http.Client
type Client struct {
    baseURL    string
    httpClient *http.Client  // shared across copies
}

func (c Client) Get(url string) (*Response, error) {
    return c.httpClient.Get(c.baseURL + url)  // uses shared client
}
```

**Slices don't require pointers:** Slices are already reference types - copying a struct with a slice copies only the 24-byte header (pointer + len + cap), NOT the underlying array. Benchmarks show slice-of-values is 24x faster than slice-of-pointers due to heap allocation overhead.

**Honest trade-offs:**

The `With*` pattern adds verbosity. `sprint = sprint.WithConsumedBuffer(0.5)` is longer than `sprint.ConsumeBuffer(0.5)`. For simple mutations, this can feel ceremonial.

Large struct copies have real cost. Go's escape analysis often helps, but not always. Profile before assuming it matters.

The Go standard library uses both patterns. `strings.Builder` mutates. `time.Time` is immutable. Context matters.

**Bottom line:** Value semantics reduce cognitive load and eliminate nil panics. The verbosity cost is usually worth the safety gain. But pointers remain the right choice when they dramatically simplify a design.

## Testing: Red/Green TDD + Khorikov Principles

Reference: Khorikov, Vladimir. "Unit Testing: Principles, Practices, and Patterns." Manning, 2020.
Summary available at: `/home/ted/projects/urma-obsidian/sources/tier2-silver/practitioner-blogs/khorikov-unit-testing-olano-summary.md`

### TDD Cycle (MANDATORY)

**CRITICAL**: Never implement before writing a failing test. If caught implementing first, STOP and revert.

1. **Red**: Write a failing test first - NO EXCEPTIONS
2. **Green**: Write minimal code to make it pass
3. **Refactor**: Clean up while keeping tests green
4. **Prune**: After refactoring, evaluate if test should be kept (see quadrants below)

### Khorikov's Four Quadrants

Categorize code BEFORE deciding what to test:

| Quadrant | Complexity | Collaborators | Test Strategy |
|----------|------------|---------------|---------------|
| **Domain/Algorithms** | High | Few | Unit test heavily (edge cases) |
| **Controllers** | Low | Many | ONE integration test per happy path |
| **Trivial** | Low | Few | **Don't test at all** |
| **Overcomplicated** | High | Many | Refactor first, then test |

### Domain/Algorithms: Unit Test Heavily
- Pure functions with business logic (e.g., `GetVarianceBounds()`, `IsWithinExpected()`)
- Domain models with calculations (e.g., `Sprint.AvgWIP()`)
- Test all edge cases with table-driven tests

### Controllers: ONE Integration Test
- **One happy path** per major workflow - not multiple!
- Plus edge cases that **cannot** be covered by unit tests
- Example: `Export()` gets ONE test that verifies files created, not separate tests for each file

**Anti-pattern**: Writing TestWriteMetrics, TestWriteTickets, TestWriteSprints separately - these are all ONE controller operation.

### Trivial Code: Don't Test
Examples of trivial code to **skip**:
- `ExportResult.Summary()` - just string formatting
- Simple getters/setters
- Constructors with no logic
- Code that just delegates

### What Makes a Test Bad (Delete It)
- Tests implementation details (e.g., checking for specific string ",5," in CSV)
- Breaks on every refactor but functionality still works
- Redundant with other tests (multiple controller tests for same operation)
- Tests trivial code that can't meaningfully fail

### Test Behavior, Not Implementation
- Test **observable outcomes**: "file exists", "has 5 lines", "returns error"
- Don't test **how**: checking specific string formats, column order, internal state
- Black-box by default: verify outputs, not steps
- Mocks only for **external boundaries** (filesystem, HTTP), never internal collaborators

### Coverage: Diagnostic Only

**How we track:**
1. Run `go test -cover ./...` at end of each phase
2. Update baseline in this file
3. Investigate any package that drops significantly or falls below 60%
4. Note: Low coverage is fine IF the code is trivial (see quadrants)

```bash
go test -cover ./...
```

**Current baseline (2026-01-03):**
| Package | Coverage | Notes |
|---------|----------|-------|
| engine | 79.1% | Domain + controller logic |
| export | 69.8% | Controller with domain helpers |
| metrics | 60.8% | Domain calculations |
| model | 28.4% | Mostly data structures (trivial) - acceptable |
| tui | 0.0% | UI controller - test via manual/integration |

Per Khorikov: Coverage is a "good negative indicator, bad positive one."
- **Below 60%**: Investigate - unless code is trivial/controller
- **High coverage**: Means nothing about quality
- **Don't target a number**: Creates perverse incentive for useless tests
- **Drops matter more than absolutes**: If a package drops 20%, investigate

### Go Test Style
- Prefer **table-driven tests** for domain algorithms
- Use descriptive test names that explain the behavior being tested
- Group related assertions in subtests with `t.Run()`

```go
func TestFoo(t *testing.T) {
    tests := []struct {
        name     string
        input    int
        expected int
    }{
        {"zero input", 0, 0},
        {"positive input", 5, 10},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Foo(tt.input)
            if got != tt.expected {
                t.Errorf("Foo(%d) = %d, want %d", tt.input, got, tt.expected)
            }
        })
    }
}
```

Run tests: `go test ./...`
Run with coverage: `go test -cover ./...`

### Integration Testing Patterns

Integration tests verify components work together. Use in-memory replacements for reliability and speed.

**In-memory replacements:**

| Dependency | Replacement | Why |
|------------|-------------|-----|
| HTTP server | `httptest.NewServer` | Random port, no conflicts |
| Database | SQLite + txdb | In-memory, DML isolated per test (DDL auto-commits) |
| File output | `bytes.Buffer` | Inspectable, no filesystem |

**Table-driven integration tests:**
```go
func TestAPI_Lifecycle(t *testing.T) {
    type want struct {
        hasTickLink        bool
        hasStartSprintLink bool
    }

    tests := []struct {
        name string
        seed int64
        want want
    }{
        {
            name: "sprint ends correctly",
            seed: 42,
            want: want{hasTickLink: false, hasStartSprintLink: true},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            registry := NewSimRegistry()
            srv := httptest.NewServer(NewRouter(registry))
            defer srv.Close()

            // ... run lifecycle ...

            if diff := cmp.Diff(got, tt.want); diff != "" {
                t.Errorf("mismatch (-got +want):\n%s", diff)
            }
        })
    }
}
```

**Use `cmp.Diff` for assertions** - single diff shows all differences.

**Comprehensive docstrings** - explain test philosophy, not just what it tests.

**Parallel testing** - tests with in-memory dependencies are naturally parallel-safe:
```go
func TestFeature(t *testing.T) {
    t.Parallel()
    srv := httptest.NewServer(handler)
    defer srv.Close()
    // ...
}
```

**Spy pattern** - capture interactions for assertion:
```go
type SpyEmailGateway struct {
    SendCalled bool
    LastTo     string
}

func (s *SpyEmailGateway) Send(to, subject, body string) error {
    s.SendCalled = true
    s.LastTo = to
    return nil
}
```

## Development Process

Our development workflow follows this sequence:

1. **Use Cases** (`/use-case-skill`): Define use cases as the origin of development
2. **Design Documents**: Create design documents from use cases
3. **Implementation Plan**: Plan the implementation with Tandem Protocol contract

Each phase produces artifacts that inform the next, ensuring traceability from user needs to code.

## Branching Strategy: Trunk-Based Development

We use trunk-based development (TBD) to minimize merge complexity and enable continuous integration.

### Core Principles

- **Single trunk**: `main` is the only long-lived branch
- **Short-lived feature branches**: If used, live < 1-2 days max
- **Small, frequent commits**: Commit directly to main when safe
- **Feature flags over branches**: Hide incomplete work behind flags, not branches
- **Always releasable**: Main should always be in a deployable state

### When to Use a Branch

| Situation | Approach |
|-----------|----------|
| Small change (< 1 day) | Commit directly to main |
| Larger feature (1-2 days) | Short-lived branch, merge quickly |
| Experimental/risky | Feature flag on main |
| Multi-day work | Break into smaller pieces that can merge daily |

### Practices

- **No long-lived feature branches**: They create merge hell
- **No release branches**: Tag releases on main instead
- **Continuous integration**: All commits trigger CI on main
- **Code review**: Use small PRs or pair programming
- **Revert over rollback**: If main breaks, revert the commit

### Feature Flags

For incomplete features that span multiple commits:
```go
if config.EnableDataExport {
    // new feature code
}
```

This lets us merge to main continuously without exposing unfinished work.

## Data Output Requirements

The simulation must produce sufficient data output to:
- Compare actual runs against theoretical predictions
- Validate variance models and incident rates
- Enable statistical analysis across multiple seeds
- Support hypothesis testing (DORA vs TameFlow)

## Project Overview

A software development simulation with two evolutionary paths:
1. Full video game
2. LLM-based laboratory for automated software development experimentation

## Benchmarks

Run benchmarks on each build per Go Development Guide §7.

```bash
go test -bench=. -benchmem ./internal/engine/
```

### Baseline (2026-01-15)

**Engine Hot Paths:**
```
BenchmarkTick-8                          63327     18776 ns/op    13448 B/op     3 allocs/op
BenchmarkTick_LargeSimulation-8          54536     20991 ns/op    18973 B/op     3 allocs/op
BenchmarkFindActiveTicketIndex-8       7165947       152.7 ns/op      0 B/op     0 allocs/op
BenchmarkVarianceCalculate-8            130179      8916 ns/op     5376 B/op     1 allocs/op
```

**Event Sourcing (Projection):**
```
BenchmarkProjection_Apply_SingleEvent-8  22574644      45.12 ns/op      0 B/op     0 allocs/op
BenchmarkProjection_ReplayFull-8            29217     36419 ns/op    560 B/op     3 allocs/op
```

Targets: SingleEvent < 1μs/op, ReplayFull (1000 events) < 1ms. Both well under targets.

**Event Sourcing (Upcaster) - 2026-01-24:**
```
BenchmarkUpcaster_Apply_NoTransform-8       5997628    214.6 ns/op    232 B/op     3 allocs/op
BenchmarkUpcaster_Apply_WithTransform-8     2554708    483.4 ns/op    464 B/op     6 allocs/op
BenchmarkUpcaster_Apply_TransitiveChain-8   1589707    740.0 ns/op    696 B/op     9 allocs/op
```

Note: WithTransform tests v1→v2 transform. TransitiveChain tests v1→v2→v3 per ES Guide §11.
Cycle detection (seen map) adds ~230ns per transform hop. Acceptable for schema evolution safety.

**FluentFP vs Loop Comparisons:**
```
BenchmarkFluentFP_ToFloat64-8          6981193       171.5 ns/op    896 B/op     1 allocs/op
BenchmarkLoop_ToFloat64-8              6461218       190.3 ns/op    896 B/op     1 allocs/op

BenchmarkFluentFP_KeepIfLen-8           273500      4315 ns/op   27264 B/op     1 allocs/op
BenchmarkLoop_FilterCount-8            2554616       475.7 ns/op      0 B/op     0 allocs/op

BenchmarkFluentFP_Fold-8               9032090       130.7 ns/op      0 B/op     0 allocs/op
BenchmarkLoop_Accumulate-8            38648880        30.49 ns/op     0 B/op     0 allocs/op

BenchmarkFluentFP_Unzip4-8              881738      1345 ns/op    3584 B/op     4 allocs/op
BenchmarkLoop_SinglePass-8             1703955       700.2 ns/op   3584 B/op     4 allocs/op
```

### Analysis

| Pattern | FluentFP | Loop | Ratio | Verdict |
|---------|----------|------|-------|---------|
| ToFloat64 | 171ns | 190ns | 0.9x | **FluentFP faster** |
| KeepIf+Len | 4315ns | 476ns | 9.1x | Loop faster (intermediate alloc) |
| Fold | 131ns | 30ns | 4.3x | Loop faster (generic overhead) |
| Unzip4 | 1345ns | 700ns | 1.9x | Loop faster (single pass) |

**Recommendations:**
- ToFloat64: Use FluentFP freely (no penalty)
- KeepIf+Len: Use loops in hot paths, FluentFP elsewhere for clarity
- Fold: Use loops for simple accumulation
- Unzip4: Use FluentFP when readability matters (1.9x acceptable)

**Future optimization candidate:** `FindActiveTicketIndex` is O(n) linear search. Consider hash map for large simulations.

### HTTP Client Benchmarks (2026-01-21)

```
BenchmarkClient_CreateSimulation-8        9374    123921 ns/op    45733 B/op    339 allocs/op
BenchmarkClient_Tick-8                    5919    402381 ns/op   550877 B/op    215 allocs/op
BenchmarkClient_Assign-8                  7780    173472 ns/op    23359 B/op    183 allocs/op
```

Target: < 1ms for local operations. All benchmarks meet target:
- CreateSimulation: ~124μs
- Tick: ~402μs (includes sprint state changes)
- Assign: ~173μs

## Persistence

Save and load simulation state for long-running experiments.

### Usage

| Key | Action |
|-----|--------|
| `Ctrl+s` | Save current state to `saves/` directory |
| `Ctrl+o` | Load most recent save file |

### Save File Format

- **Format:** Go's `encoding/gob` (binary, efficient)
- **Extension:** `.sds` (simulation data state)
- **Location:** `saves/` directory (auto-created)

### Schema Versioning

Save files include a schema version for forward compatibility:

```go
type SaveFile struct {
    Version   int              // Schema version (currently 1)
    Timestamp time.Time        // When saved
    Name      string           // Auto-generated or user-provided
    State     SimulationState  // Full state
}

type SimulationState struct {
    Simulation model.Simulation   // Core simulation
    DORA       metrics.DORAMetrics // DORA with history
    Fever      metrics.FeverChart  // Fever with history
}
```

### Migration

When schema changes are needed:
1. Increment `CurrentVersion` in `persistence/schema.go`
2. Add migration function in `persistence/migrate.go`
3. `Load()` automatically runs migration chain

### API

```go
import "github.com/binaryphile/sofdevsim-2026/internal/persistence"

// Save simulation state
err := persistence.Save(path, name, sim, tracker)

// Load simulation state
sim, tracker, err := persistence.Load(path)

// List available saves
saves, err := persistence.ListSaves(dir)

// Generate save path with sanitized name
path := persistence.GenerateSavePath(dir, name)
```

## Event Versioning

Events are immutable once stored. Schema evolution requires careful handling since old events remain in the store forever.

### Philosophy

We use **upcasting**: transform old event versions to current schema on read. This approach:
- Keeps storage simple (no version numbers embedded in events)
- Centralizes evolution logic in one place
- Applies transformations lazily (on Replay/Subscribe, not on store migration)

### Schema Evolution Strategies (ES Guide §11)

| Strategy | Description | Trade-off |
|----------|-------------|-----------|
| **Upcasting** (our approach) | Transform old events on read | Simple storage, centralized logic |
| **Versioned types** | `TicketAssignedV1`, `TicketAssignedV2` | Explicit, but type proliferation |
| **Weak schema** | JSON with optional fields | Flexible, but runtime errors |
| **Copy-transform** | Migrate entire store | Clean slate, but requires downtime |

### Rule of Thumb

- **Add fields freely**: Old events decode with zero values (use defaults in code)
- **Rename/remove fields**: Requires upcast function to transform old → new

### Typed vs Raw Storage

Our implementation uses **typed events** (gob-encoded Go structs), not raw bytes:

| Aspect | Our Approach | ES Guide Approach |
|--------|--------------|-------------------|
| Storage | gob-encoded typed events | JSON/bytes |
| Upcast input | `Event` interface | `RawEvent` (bytes) |
| Upcast output | `Event` interface | Typed `Event` |
| Implication | Old event types must remain in codebase | Can delete old type definitions |

For simulation scope, keeping old types is acceptable. Production systems with decades of events might prefer raw storage.

### Upcast Registry

Transforms are registered in `internal/events/upcasting.go`:

```go
func newUpcaster() Upcaster {
    return Upcaster{
        transforms: map[string]func(Event) Event{
            // Add transforms here as schema evolves:
            // "TicketAssignedV1": upcastTicketAssignedV1ToV2,
        },
    }
}
```

### Example Upcast Function

```go
// upcastTicketAssignedV1ToV2 transforms old assignment events.
// V1 had Developer string field, V2 has DeveloperID + DeveloperName.
func upcastTicketAssignedV1ToV2(evt Event) Event {
    v1 := evt.(TicketAssignedV1)
    return TicketAssignedV2{
        baseEvent:     v1.baseEvent,
        TicketID:      v1.TicketID,
        DeveloperID:   v1.Developer,           // Map old field
        DeveloperName: "[migrated]",           // Default for missing data
    }
}
```

### Where Upcasts Apply

- **Store.Replay**: Returns upcasted events for projection rebuild
- **Store.Subscribe**: Delivers upcasted events to live subscribers

Raw events remain unchanged in storage—upcasting is read-side only.

### Event Metadata

Infrastructure for correlation and causation tracking exists in Header fields but is not yet wired into production operations.

**Available API:**

| Method | Purpose | Status |
|--------|---------|--------|
| `Engine.SetTrace(tc)` | Set correlation context | Test-only |
| `Engine.ClearTrace()` | Clear correlation context | Test-only |
| `Engine.CurrentTrace()` | Get current trace context | Test-only |
| `evt.WithCausedBy(id)` | Link causation chain | Test-only |

**Usage example:**
```go
// Correlation: group related events under one trace
eng = eng.SetTrace(events.TraceContext{TraceID: "req-123", SpanID: "op-1"})
eng = must.Get2(eng.AddTicket(ticket))  // Event includes trace context
eng = eng.ClearTrace()

// Causation: link event chains
created := events.NewTicketCreated("sim-1", 0, "t-1", "Title", 3.0, model.WellUnderstood)
assigned := events.NewTicketAssigned("sim-1", 1, "t-1", "dev-1", time.Now()).
    WithCausedBy(created.EventID())
```

**Header Fields:**

| Field | Purpose |
|-------|---------|
| `Trace` | Correlation ID linking related events |
| `Span` | Current operation identifier |
| `ParentSpan` | Parent operation for nested traces |
| `CausedByID` | ID of event that caused this event |

**Future:** Wire `SetTrace` into API request handlers and `WithCausedBy` into Engine operations that trigger cascading events.
