// Command whipbench benchmarks WebRTC servers over WHIP (ingest) and WHEP
// (playback) with native clients — no browser — and writes a report that compares
// across servers.
//
//	whipbench publish --whip URL [--codec vp8|h264] [--duration 0]
//	whipbench view    --whep URL [-n 10] [--ramp 0s] [--ramp-offset-seed N] [--duration 30s] [--out DIR]
//	whipbench run     scenario.json [--out DIR] [--metrics ADDR] [--ramp-offset-seed N]
//	whipbench version
//
// Exit status: 0 a valid run, 1 an error, 2 a usage error, 3 a run with no verdict.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Allan-Nava/whipbench"
	"github.com/Allan-Nava/whipbench/internal/clip"
	"github.com/Allan-Nava/whipbench/internal/publisher"
	"github.com/Allan-Nava/whipbench/internal/report"
	"github.com/Allan-Nava/whipbench/internal/rtc"
	"github.com/Allan-Nava/whipbench/internal/runner"
	"github.com/Allan-Nava/whipbench/internal/scenario"
	"github.com/Allan-Nava/whipbench/internal/version"
	"github.com/Allan-Nava/whipbench/internal/whip"
)

const (
	exitOK        = 0
	exitError     = 1
	exitUsage     = 2
	exitNoVerdict = 3
)

const usage = `whipbench — WebRTC benchmark over WHIP and WHEP, native clients, no browser.

Usage:
  whipbench publish --whip URL [flags]       publish the synthetic clip in a loop
  whipbench view    --whep URL [flags]       N concurrent viewers, then a report
  whipbench run     SCENARIO.json [flags]    one publisher plus viewers on a ramp
  whipbench version

Run "whipbench <command> -h" for the flags of a command.

Only point whipbench at servers you run, at a managed service on your own
account and within its terms, or with the written permission of whoever runs
it (docs/load-testing-etiquette.md). The report records endpoint hosts only,
never paths, query strings or tokens.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return exitUsage
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch args[0] {
	case "publish":
		return cmdPublish(ctx, args[1:], stdout, stderr)
	case "view":
		return cmdView(ctx, args[1:], stdout, stderr)
	case "run":
		return cmdRun(ctx, args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, "whipbench", version.String())
		return exitOK
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return exitOK
	default:
		fmt.Fprintf(stderr, "whipbench: unknown command %q\n\n%s", args[0], usage)
		return exitUsage
	}
}

// parse parses flags that may come before, after or between positional
// arguments, which the standard flag package alone does not allow.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("whipbench "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func bearerFrom(env string) (string, error) {
	if env == "" {
		return "", nil
	}
	v := os.Getenv(env)
	if v == "" {
		return "", fmt.Errorf("--bearer-env %s: the variable is empty or unset", env)
	}
	return v, nil
}

// rampOffsetFlags adds --ramp-offset-seed and --ramp-offset-max (WB-8) and returns
// a function that applies whichever of them were given to sc, so `run` can let
// them override a scenario file the way --metrics does.
func rampOffsetFlags(fs *flag.FlagSet) func(sc *scenario.Scenario) {
	var seed *int64
	fs.Func("ramp-offset-seed", "offset each viewer's start by a seeded random amount below --ramp-offset-max; the report records the seed and every offset", func(v string) error {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return errors.New("want an integer")
		}
		seed = &n
		return nil
	})
	bound := fs.Duration("ramp-offset-max", 0, "bound of the seeded start offset; needs --ramp-offset-seed (default 1s, the clip's GOP)")
	return func(sc *scenario.Scenario) {
		if seed != nil {
			sc.RampOffsetSeed = seed
		}
		if *bound != 0 {
			sc.RampOffsetMaxSeconds = bound.Seconds()
		}
	}
}

func loadClip(codec, path string) (*clip.Clip, error) {
	data := map[string][]byte{clip.VP8: whipbench.ClipVP8, clip.H264: whipbench.ClipH264}[codec]
	if path != "" {
		b, err := os.ReadFile(path) //nolint:gosec // the user names the file
		if err != nil {
			return nil, err
		}
		data = b
	}
	return clip.Load(codec, data)
}

func logger(w io.Writer) func(string, ...any) {
	return func(format string, a ...any) {
		fmt.Fprintf(w, "%s  %s\n", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
	}
}

func cmdPublish(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlags("publish", stderr)
	endpoint := fs.String("whip", "", "WHIP endpoint URL (required)")
	codec := fs.String("codec", "vp8", "clip codec: vp8 or h264")
	clipPath := fs.String("clip", "", "stream this file instead of the embedded clip (IVF for vp8, Annex-B at 30 fps for h264)")
	duration := fs.Duration("duration", 0, "stop after this long; 0 publishes until interrupted")
	bearerEnv := fs.String("bearer-env", "", "name of an environment variable holding a bearer token")
	loopback := fs.Bool("include-loopback", false, "add loopback ICE candidates (a server in a local container)")
	noStamp := fs.Bool("no-stamp", false, "do not stamp send times in abs-capture-time")
	if _, err := parse(fs, args); err != nil {
		return exitUsage
	}
	if *endpoint == "" {
		fmt.Fprintln(stderr, "whipbench publish: --whip is required")
		return exitUsage
	}
	bearer, err := bearerFrom(*bearerEnv)
	if err != nil {
		fmt.Fprintln(stderr, "whipbench publish:", err)
		return exitUsage
	}
	c, err := loadClip(*codec, *clipPath)
	if err != nil {
		fmt.Fprintln(stderr, "whipbench publish:", err)
		return exitUsage
	}
	logf := logger(stderr)
	logf("publishing %s (%d frames, %.0f fps, keyframe every %d frames) to %s",
		c.Codec, len(c.Frames), float64(time.Second)/float64(c.FrameDuration()), c.KeyframeInterval(), whip.Host(*endpoint))
	pub, err := publisher.Connect(ctx, publisher.Config{
		WHIP: *endpoint, Bearer: bearer, Clip: c, RTC: rtc.Options{IncludeLoopback: *loopback}, NoStamp: *noStamp,
	})
	if err != nil {
		fmt.Fprintln(stderr, "whipbench publish:", err)
		return exitError
	}
	logf("connected in %.0f ms; send-time stamp negotiated: %v", pub.Result().ConnectMs, pub.Stamped())
	sctx := ctx
	if *duration > 0 {
		var cancel context.CancelFunc
		sctx, cancel = context.WithTimeout(ctx, *duration)
		defer cancel()
	}
	res := pub.Stream(sctx)
	fmt.Fprintf(stdout, "published %d frames, %d packets, %d bytes in %.1f s (%d loops, %d schedule slips)\n",
		res.FramesSent, res.PacketsSent, res.BytesSent, res.SendingSeconds, res.Loops, res.ScheduleSlips)
	if res.Error != "" {
		fmt.Fprintln(stderr, "whipbench publish:", res.Error)
		return exitError
	}
	return exitOK
}

func cmdView(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlags("view", stderr)
	var sc scenario.Scenario
	fs.StringVar(&sc.WHEP, "whep", "", "WHEP endpoint URL (required)")
	fs.IntVar(&sc.Viewers, "n", 10, "number of concurrent viewers")
	ramp := fs.Duration("ramp", 0, "spread the viewers' starts over this long")
	hold := fs.Duration("duration", 30*time.Second, "how long every viewer stays after the ramp")
	join := fs.Duration("join-timeout", 10*time.Second, "POST → first complete keyframe, or the viewer failed")
	fs.StringVar(&sc.Name, "name", "", "a name for the run, used in the report title and file names")
	fs.StringVar(&sc.BearerEnv, "bearer-env", "", "name of an environment variable holding a bearer token")
	fs.StringVar(&sc.Metrics, "metrics", "", "serve Prometheus metrics on this address during the run, e.g. 127.0.0.1:9464")
	fs.BoolVar(&sc.IncludeLoopback, "include-loopback", false, "add loopback ICE candidates (a server in a local container)")
	fs.BoolVar(&sc.LoopbackOnly, "loopback-only", false, "gather ICE candidates on loopback only (a server on this machine)")
	applyOffset := rampOffsetFlags(fs)
	out := fs.String("out", ".", "directory for the JSON and Markdown report")
	if _, err := parse(fs, args); err != nil {
		return exitUsage
	}
	sc.RampSeconds, sc.HoldSeconds, sc.JoinTimeoutSeconds = ramp.Seconds(), hold.Seconds(), join.Seconds()
	applyOffset(&sc)
	sc.Normalise()
	if err := sc.Validate(); err != nil {
		fmt.Fprintln(stderr, "whipbench view:", err)
		return exitUsage
	}
	return execute(ctx, "view", sc, nil, *out, stdout, stderr)
}

func cmdRun(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	fs := newFlags("run", stderr)
	out := fs.String("out", ".", "directory for the JSON and Markdown report")
	metricsAddr := fs.String("metrics", "", "serve Prometheus metrics on this address (overrides the scenario)")
	applyOffset := rampOffsetFlags(fs) // overrides the scenario's rampOffset keys
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage
	}
	if len(pos) != 1 {
		fmt.Fprintln(stderr, "whipbench run: want exactly one scenario file")
		return exitUsage
	}
	sc, err := scenario.Load(pos[0])
	if err != nil {
		fmt.Fprintln(stderr, "whipbench run:", err)
		return exitUsage
	}
	if *metricsAddr != "" {
		sc.Metrics = *metricsAddr
	}
	applyOffset(&sc)
	sc.Normalise()
	if err := sc.Validate(); err != nil {
		fmt.Fprintln(stderr, "whipbench run:", err)
		return exitUsage
	}
	var c *clip.Clip
	if sc.WHIP != "" {
		if c, err = loadClip(sc.Codec, ""); err != nil {
			fmt.Fprintln(stderr, "whipbench run:", err)
			return exitError
		}
	}
	return execute(ctx, "run", sc, c, *out, stdout, stderr)
}

func execute(ctx context.Context, command string, sc scenario.Scenario, c *clip.Clip, out string, stdout, stderr io.Writer) int {
	bearer, err := bearerFrom(sc.BearerEnv)
	if err != nil {
		fmt.Fprintf(stderr, "whipbench %s: %v\n", command, err)
		return exitUsage
	}
	rep, err := runner.Run(ctx, runner.Options{Command: command, Scenario: sc, Bearer: bearer, Clip: c, Logf: logger(stderr)})
	if err != nil {
		fmt.Fprintf(stderr, "whipbench %s: %v\n", command, err)
		return exitError
	}
	paths, err := write(rep, out)
	if err != nil {
		fmt.Fprintf(stderr, "whipbench %s: %v\n", command, err)
		return exitError
	}
	a := rep.Aggregate
	fmt.Fprintf(stdout, "%s\n", a.Verdict)
	if a.Valid {
		fmt.Fprintf(stdout, "join (first keyframe) p50 %.0f ms, p95 %.0f ms; loss %.3f%%; ", a.FirstKeyframeMs.P50, a.FirstKeyframeMs.P95, a.LossTotal)
		if a.PacketTransit.Available && a.PacketTransit.Ms != nil {
			fmt.Fprintf(stdout, "packet transit p50 %.1f ms, p99 %.1f ms\n", a.PacketTransit.Ms.P50, a.PacketTransit.Ms.P99)
		} else {
			fmt.Fprintf(stdout, "packet transit unavailable: %s\n", a.PacketTransit.Reason)
		}
	}
	for _, p := range paths {
		fmt.Fprintln(stdout, "wrote", p)
	}
	if !a.Valid {
		return exitNoVerdict
	}
	return exitOK
}

var unsafeName = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func write(rep *report.Report, dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	stem := "whipbench-" + rep.StartedAt.UTC().Format("20060102T150405Z")
	if n := strings.Trim(unsafeName.ReplaceAllString(rep.Scenario.Name, "-"), "-"); n != "" {
		stem += "-" + n
	}
	js, err := rep.JSON()
	if err != nil {
		return nil, err
	}
	jp, mp := filepath.Join(dir, stem+".json"), filepath.Join(dir, stem+".md")
	if err := os.WriteFile(jp, js, 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(mp, []byte(rep.Markdown()), 0o600); err != nil {
		return nil, errors.Join(err, os.Remove(jp))
	}
	return []string{jp, mp}, nil
}
