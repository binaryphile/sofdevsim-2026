# FluentFP Guide

Complete API reference for `github.com/binaryphile/fluentfp`. For quick reference, see CLAUDE.md.

## slice Package - Complete API

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

// Standalone functions
slice.Find[T](ts []T, fn func(T) bool) option.Basic[T]  // First matching element
```

## slice Patterns

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

## Chain Formatting

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

Setup (`slice.From`, `slice.MapTo[R]`) doesn't count—only chained methods count.

## option Package

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

## option Patterns

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

## Pseudo-Option Conventions

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

## Prefer Options Over Nil Pointers

For optional values, prefer `option.Basic[T]` over `*T`:

| Pattern | Problem | Solution |
|---------|---------|----------|
| `*T` for "maybe absent" | Nil checks scattered, panic risk | `option.Basic[T]` |
| `if ptr != nil` | Easy to forget, silent bugs | `.Get()` forces handling |
| `*ptr` access | Panics if nil | `.OrZero()` or `.Or(default)` |

**Use pointers for:** `sync.Mutex` fields, interface requirements, external handles, or profiled hot paths >10MB.
**Use options for:** "This value may not exist" semantics.

## Prefer Options Over Comma-Ok Returns

Our methods should not return `(T, bool)` comma-ok for optional values. Comma-ok is for *consuming* Go builtins (map access, type assertions, channel receives), not for our own APIs.

| Pattern | Problem | Solution |
|---------|---------|----------|
| `Get(id) (T, bool)` | Hidden option, requires wrapping | `GetOption(id) option.Basic[T]` |
| `option.New(x.Get(id))` | Extra ceremony at call site | `x.GetOption(id).KeepOkIf(...)` |

**Naming convention:** Methods returning `option.Basic[T]` use `Option` suffix: `GetAnimationOption`, `GetInstanceOption`.

```go
// BAD: comma-ok return (hidden option)
func (s State) GetAnimation(id string) (Animation, bool)

// GOOD: explicit option return
func (s State) GetAnimationOption(id string) option.Basic[Animation]
```

Call sites become chainable:
```go
// Before: wrapping required
option.New(state.GetAnimation(id)).KeepOkIf(Animation.IsActive).Call(...)

// After: direct chaining
state.GetAnimationOption(id).KeepOkIf(Animation.IsActive).Call(...)
```

## Lifted Functions for Side Effects

Use `option.Lift` to create named functions that accept options. The lifted function executes only when the option is ok:

```go
// Define a lifted function with a descriptive name
recordCompletion := option.Lift(func(anim Animation) {
    projection = projection.Record(SomeEvent{ID: anim.ID})
})

// Call with the option - reads as natural function call
recordCompletion(state.GetAnimationOption(id))
```

This pattern creates a named function at definition time, so call sites are clean function calls with option arguments.

**Avoid double-execute syntax:**
```go
// BAD: option.Lift(fn)(opt) - hard to read, anonymous
option.Lift(func(a Animation) { ... })(state.GetAnimationOption(id))

// GOOD: named function called with option
recordCompletion := option.Lift(func(a Animation) { ... })
recordCompletion(state.GetAnimationOption(id))
```

## either Package

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

## either Patterns

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

## Either vs Option

| Pattern | Use | Example |
|---------|-----|---------|
| `option.Basic[T]` | Value may be absent | Database nullable field |
| `either.Either[L, R]` | One of two distinct states | Mode A or Mode B |

Option is for "maybe nothing." Either is for "definitely something, but which one?"

## must Package

```go
import "github.com/binaryphile/fluentfp/must"

must.Get(t T, err error) T                    // Return or panic
must.Get2(t T, t2 T2, err error) (T, T2)      // 3-return variant
must.BeNil(err error)                         // Panic if error
must.Getenv(key string) string                // Env var or panic
must.Of(fn func(T) (R, error)) func(T) R      // Wrap fallible func
```

## must Patterns

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
eng = must.Get(eng.StartSprint())           // Single-user: no concurrent conflicts
eng, events = must.Get2(eng.Tick())         // 3-return: (Engine, []Event, error)
sprint := sim.CurrentSprintOption.MustGet() // Option has MustGet() for invariants
```

**When to use must vs error handling:**
- `must.Get`: Error indicates a bug (invariant violation), not a runtime condition
- Error handling: Error is expected and recoverable (user input, network, file I/O)
- If you think "this error can never happen here", use `must` to enforce that invariant

## value Package

Value-first conditional selection. Use when rendering a value with a fallback, not for branching logic.

```go
import "github.com/binaryphile/fluentfp/value"

value.Of(t T).When(cond bool).Or(fallback T) T
```

Reads as plain English: "value of x when condition, or fallback"

**Patterns:**
```go
// Conditional value with fallback
days := value.Of(sim.CurrentTick).When(sim.CurrentTick < 7).Or(7)

// Status rendering
status := value.Of(GreenStyle.Render("[idle]")).When(dev.IsIdle()).Or(YellowStyle.Render("[busy]"))
```

**When to use:** Conditional value selection with a fallback. Not for branching logic with side effects.

## lof Package (Lower-Order Functions)

```go
import "github.com/binaryphile/fluentfp/lof"

lof.Println(s string)      // Wraps fmt.Println for Each
lof.Len(ts []T) int        // Wraps len
```

## pair Package (Tuples)

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

**Patterns:**
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

## Fold and Unzip

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
leadTimes, deployFreqs, mttrs, cfrs := slice.Unzip4(history,
    HistoryPoint.GetLeadTimeAvg,
    HistoryPoint.GetDeployFrequency,
    HistoryPoint.GetMTTR,
    HistoryPoint.GetChangeFailRate,
)
```

## Named Functions in FluentFP Chains

**This guidance applies to FluentFP chains only.** For simple if statements, inline conditions are clearer.

**For FluentFP chains**, prefer named functions over inline lambdas:

```go
// GOOD: Named predicate with leading comment
// completedAfterCutoff returns true if ticket was completed after the cutoff tick.
completedAfterCutoff := func(t Ticket) bool { return t.CompletedTick >= cutoff }
count := slice.From(tickets).KeepIf(completedAfterCutoff).Len()

// BAD: Inline lambda in chain - harder to read
count := slice.From(tickets).KeepIf(func(t Ticket) bool { return t.CompletedTick >= cutoff }).Len()
```

**Preference hierarchy:**
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

**All named functions get leading comments. Single-expression functions go on one line:**

```go
// completedAfterCutoff returns true if ticket was completed after the cutoff tick.
completedAfterCutoff := func(t Ticket) bool { return t.CompletedTick >= cutoff }

// sumDuration adds two durations.
sumDuration := func(a, b time.Duration) time.Duration { return a + b }
```

**Locality:** Define named functions close to first usage, not at package level.

## Method Expressions (preferred)

When a type has a method matching the required signature, use it directly:
```go
// Method expression: reads as English, no function body to parse
slice.From(developers).KeepIf(Developer.IsIdle)

// Inline anonymous: reader must parse lambda syntax
slice.From(developers).KeepIf(func(d Developer) bool { return d.IsIdle() })
```

**Critical: Use value receivers for read-only methods.** Method expressions only work when receiver type matches slice element type:

```go
// Works with slice.From
func (u User) IsActive() bool { return u.Active }

// Doesn't work - (*User).IsActive expects *User, not User
func (u *User) IsActive() bool { return u.Active }
```

**Design rule:** Value receivers by default, pointer receivers only when mutating.

**Adding accessor methods for FluentFP:** When a type lacks methods for field extraction, add accessor methods:

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

**When to add accessors:** Add `Get[FieldName]` methods when the type is used in FluentFP operations more than once.

## Predicate Naming Patterns

| Pattern | When to use | Example |
|---------|-------------|---------|
| `Is[Condition]` | Simple check, subject obvious | `IsValidMAC` |
| `[Subject]Is[Condition]` | State check on specific type | `SliceOfScansIsEmpty` |
| `[Subject]Has[Condition](params)` | Parameterized predicate factory | `DeviceHasHWVersion("EX12")` |
| `Type.Is[Condition]` | Method expression | `Device.IsActive` |

## Why Prefer FluentFP for Data Transformation

**Concrete example - field extraction:**

```go
// FluentFP: extract percent values from history
return slice.From(f.History).ToFloat64(FeverSnapshot.GetPercentUsed)

// Loop: four concepts interleaved
var result []float64                           // 1. variable declaration
for _, s := range f.History {                  // 2. iteration mechanics
    result = append(result, s.PercentUsed)     // 3. append mechanics
}
return result                                  // 4. return
```

The loop forces you to think about *how*. FluentFP expresses *what*.

**Performance note:** Mapping operations match or beat loops. Filter operations (KeepIf) allocate intermediate slices—use loops for filter+count in hot paths.

## When Loops Are Still Necessary

1. **Channel consumption** - `for r := range chan` has no FP equivalent
2. **Complex control flow** - break/continue/early return within loop
3. **Index-dependent logic** - when you need `i` for more than just indexing

See [fluentfp/slice/README.md](https://github.com/binaryphile/fluentfp/blob/develop/slice/README.md#when-loops-are-still-necessary) for detailed examples.
