package codemode

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func run(t *testing.T, code string, opts Options) Result {
	t.Helper()
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}
	return testEngine(t).Execute(context.Background(), code, opts)
}

func texts(r Result) string {
	var parts []string
	for _, o := range r.Output {
		if o.Type == "text" {
			parts = append(parts, o.Text)
		}
	}
	return strings.Join(parts, "|")
}

func TestReturnValueAndOutput(t *testing.T) {
	r := run(t, `text("hello"); console.log({a: 1}); console.error("oops"); return 1 + 2;`, Options{})
	if !r.OK || string(r.Value) != "3" {
		t.Fatalf("result %+v", r)
	}
	if got := texts(r); got != `hello|{"a":1}|oops` {
		t.Fatalf("output %q", got)
	}
	if u := run(t, `text("x")`, Options{}); !u.OK || u.Value != nil {
		t.Fatalf("undefined return: %+v", u)
	}
}

// Tool calls are JSON round trips, run concurrently (Promise.all), and a
// failing tool rejects with its message; scripts can catch it.
func TestToolsCallsConcurrencyAndErrors(t *testing.T) {
	var inFlight, peak int32
	slow := Tool{Name: "slow-echo", Description: "Echo after a pause", Execute: func(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		return args, nil
	}}
	fail := Tool{Name: "fail", Execute: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return nil, errors.New("tool exploded")
	}}
	r := run(t, `
		const results = await Promise.all([1, 2, 3].map((n) => tools.slow_echo({ n })));
		let caught = "";
		try { await tools.fail({}); } catch (e) { caught = e.message; }
		return { sum: results.reduce((a, r) => a + r.n, 0), caught, names: ALL_TOOLS.map((t) => t.name), same: tools["slow-echo"] === tools.slow_echo };`,
		Options{Tools: []Tool{slow, fail}})
	if !r.OK {
		t.Fatalf("error %+v", r.Error)
	}
	var v struct {
		Sum    int
		Caught string
		Names  []string
		Same   bool
	}
	if err := json.Unmarshal(r.Value, &v); err != nil {
		t.Fatal(err)
	}
	if v.Sum != 6 || v.Caught != "tool exploded" || strings.Join(v.Names, ",") != "slow_echo,fail" || !v.Same {
		t.Fatalf("value %+v", v)
	}
	if atomic.LoadInt32(&peak) < 2 {
		t.Fatalf("tool calls did not run concurrently (peak %d)", peak)
	}
	if len(r.Calls) != 4 || r.Calls[3].Name != "fail" || r.Calls[3].Status != "error" || r.Calls[0].Status != "ok" {
		t.Fatalf("calls %+v", r.Calls)
	}
}

func TestScriptErrorsAndSyntax(t *testing.T) {
	r := run(t, `throw new TypeError("bad input")`, Options{})
	if r.OK || r.Error.Kind != ErrorScript || r.Error.Name != "TypeError" || r.Error.Message != "bad input" || !strings.HasPrefix(r.Error.Stack, "TypeError: bad input") {
		t.Fatalf("thrown: %+v", r.Error)
	}
	s := run(t, `return (`, Options{})
	if s.OK || s.Error.Kind != ErrorScript || s.Error.Name != "SyntaxError" {
		t.Fatalf("syntax: %+v", s.Error)
	}
}

// exit() ends successfully at once, keeping output; a promise nothing can
// settle fails instead of hanging.
func TestExitAndStall(t *testing.T) {
	r := run(t, `text("before"); exit(); text("after");`, Options{})
	if !r.OK || texts(r) != "before" {
		t.Fatalf("exit: %+v %q", r.Error, texts(r))
	}
	start := time.Now()
	s := run(t, `await new Promise(() => {}); return 1;`, Options{})
	if s.OK || s.Error.Kind != ErrorScript || time.Since(start) > 3*time.Second {
		t.Fatalf("stall: %+v after %s", s.Error, time.Since(start))
	}
}

func TestStoreRoundTripAndLimits(t *testing.T) {
	r := run(t, `
		const runs = (load("runs") ?? 0) + 1;
		store("runs", runs);
		const obj = load("obj"); obj.mutated = true;
		store("gone", undefined);
		let tooBig = "";
		try { store("big", "x".repeat(300 * 1024)); } catch (e) { tooBig = e.name; }
		return { runs, untouched: load("obj").mutated === undefined, tooBig };`,
		Options{Store: map[string]json.RawMessage{"runs": json.RawMessage("2"), "obj": json.RawMessage(`{"k":1}`), "gone": json.RawMessage("1")}})
	if !r.OK {
		t.Fatalf("error %+v", r.Error)
	}
	if string(r.Value) != `{"runs":3,"untouched":true,"tooBig":"RangeError"}` {
		t.Fatalf("value %s", r.Value)
	}
	if string(r.StoreWrites.Set["runs"]) != "3" || len(r.StoreWrites.Delete) != 1 || r.StoreWrites.Delete[0] != "gone" {
		t.Fatalf("writes %+v", r.StoreWrites)
	}
	if f := run(t, `store("k", 1); throw new Error("no")`, Options{}); f.OK || len(f.StoreWrites.Set) != 0 {
		t.Fatalf("failed run must report no writes: %+v", f.StoreWrites)
	}
}

// Infinite loops end at the deadline (interrupt), and cancelling the caller's
// context aborts; deep recursion is a catchable RangeError.
func TestTimeoutAbortAndRecursion(t *testing.T) {
	e := testEngine(t)
	start := time.Now()
	r := e.Execute(context.Background(), `while (true) {}`, Options{Timeout: 300 * time.Millisecond})
	if r.OK || r.Error.Kind != ErrorTimeout || time.Since(start) > 5*time.Second {
		t.Fatalf("timeout: %+v after %s", r.Error, time.Since(start))
	}
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	a := e.Execute(ctx, `while (true) {}`, Options{Timeout: 10 * time.Second})
	if a.OK || a.Error.Kind != ErrorAborted {
		t.Fatalf("abort: %+v", a.Error)
	}
	rec := run(t, `function f(n) { return f(n + 1); } try { f(0); } catch (e) { return e.name; }`, Options{})
	if !rec.OK || string(rec.Value) != `"RangeError"` {
		t.Fatalf("recursion: %+v %s", rec.Error, rec.Value)
	}
}

func TestImagesAndGlobals(t *testing.T) {
	png := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	classify := Global{Name: "models.classify", Execute: func(_ context.Context, args json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`"label:` + strings.Trim(string(args), `"`) + `"`), nil
	}}
	r := run(t, `image("data:image/png;base64,`+png+`"); return await models.classify("cat");`, Options{Globals: []Global{classify}})
	if !r.OK || string(r.Value) != `"label:cat"` {
		t.Fatalf("globals: %+v %s", r.Error, r.Value)
	}
	if len(r.Output) != 1 || r.Output[0].Type != "image" || r.Output[0].MimeType != "image/png" || r.Output[0].Data != png {
		t.Fatalf("image output %+v", r.Output)
	}
	if bad := run(t, `image("https://example.com/x.png")`, Options{}); bad.OK {
		t.Fatal("remote image URL accepted")
	}
}

func TestIdentifier(t *testing.T) {
	for in, want := range map[string]string{"mcp__docs__search": "mcp__docs__search", "my-tool": "my_tool", "9lives": "_lives", "": "_", "a.b c": "a_b_c"} {
		if got := Identifier(in); got != want {
			t.Fatalf("%q -> %q, want %q", in, got, want)
		}
	}
}

// Pi 1.0's prelude names close matches for a missing tool, and `in` probes
// still work.
func TestMissingToolNamesCloseMatches(t *testing.T) {
	tools := []Tool{{Name: "bash", Description: "Run", Execute: func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`"ok"`), nil
	}}}
	res := testEngine(t).Execute(context.Background(), `if ("Bash" in tools) return "bad"; await tools.Bash({});`, Options{Tools: tools})
	if res.OK || res.Error == nil || !strings.Contains(res.Error.Message, "tools.Bash does not exist. Did you mean tools.bash?") {
		t.Fatalf("%+v %+v", res, res.Error)
	}
}
