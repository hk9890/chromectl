package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hk9890/chromectl/internal/client"
	"github.com/hk9890/chromectl/internal/proto"
)

// newFlagSet exits 2 on a bad flag, as main does for an unknown command, and
// 0 on -h. Both print the usage once, to stderr.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	fs.Usage = func() { _, _ = fmt.Fprint(fs.Output(), usage) }
	return fs
}

// flags are the flags of a command that calls a host: --timeout, and --json
// where the command prints a result.
type flags struct {
	*flag.FlagSet
	json    *bool
	timeout *time.Duration
}

func newFlags(name string, withJSON bool) flags {
	f := flags{FlagSet: newFlagSet(name), json: new(bool)}
	if withJSON {
		f.json = f.Bool("json", false, "print JSON")
	}
	f.timeout = f.Duration("timeout", client.Timeout, "how long each call to a host waits")
	return f
}

// parse reads args and sets how long each call waits.
func (f flags) parse(args []string) error {
	_ = f.Parse(args)
	if *f.timeout <= 0 {
		return badUsage("--timeout must be more than zero, not %s", *f.timeout)
	}
	client.Timeout = *f.timeout
	return nil
}

// usageError is a call chromectl cannot make sense of. It exits 2.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func badUsage(format string, args ...any) error {
	return usageError{fmt.Sprintf(format, args...)}
}

// noHostForError is a profile no host serves. It is client.ErrNoHost for one
// profile: nothing reached Chrome.
type noHostForError struct{ profile string }

func (e noHostForError) Error() string {
	return fmt.Sprintf("no chromectl host for profile %s: is that profile still open?", e.profile)
}

func (e noHostForError) Is(target error) bool { return target == client.ErrNoHost }

// incompleteError is a fan-out that printed what answered while a host did
// not. It exits as its first failure does.
type incompleteError struct {
	what  string
	first error
}

func (e incompleteError) Error() string {
	return "the " + e.what + " is incomplete: a host above did not answer"
}

func (e incompleteError) Unwrap() error { return e.first }

// exitCode maps an error to the code the caller branches on.
func exitCode(err error) int {
	var bad usageError
	var noReply *client.NoReplyError
	switch {
	case errors.As(err, &bad):
		return 2
	case errors.As(err, &noReply):
		return 4
	case errors.Is(err, client.ErrNoHost):
		return 5
	}
	return 1
}

var compositeID = regexp.MustCompile(`^(` + proto.ProfileID + `):([0-9]+)$`)

// parseID reads a tab or window id as list prints it: the profile that owns
// it, a colon, and Chrome's number, which starts at 1.
func parseID(kind, arg string) (profile string, id int, err error) {
	m := compositeID.FindStringSubmatch(arg)
	if m != nil {
		id, err = strconv.Atoi(m[2])
	}
	if m == nil || err != nil || id < 1 {
		return "", 0, badUsage("%s id %q is not of the form <profile>:<number> that list prints", kind, arg)
	}
	return m[1], id, nil
}

func composite(profile string, id int) string { return fmt.Sprintf("%s:%d", profile, id) }

func hosts() ([]client.Host, error) {
	dir, err := client.SocketDir()
	if err != nil {
		return nil, err
	}
	return client.Hosts(dir)
}

// hostFor returns the host of profile, or the error that says why there is
// none.
func hostFor(hs []client.Host, profile string) (client.Host, error) {
	for _, h := range hs {
		if h.Profile == profile && h.Conflict != "" {
			return h, errors.New(h.Conflict)
		}
		if h.Profile == profile {
			return h, nil
		}
	}
	if len(hs) == 0 {
		return client.Host{}, client.ErrNoHost
	}
	if err := staleReason(hs); err != nil {
		return client.Host{}, err
	}
	return client.Host{}, noHostForError{profile}
}

// staleReason returns the answer of a stale host, which cannot serve and
// cannot say which profile it is, so it may be the one the caller wants.
func staleReason(hs []client.Host) error {
	for _, h := range hs {
		if h.Profile != "" {
			continue
		}
		if err := client.Call(h.Path, proto.Request{Cmd: "list"}, nil); err != nil && !errors.Is(err, client.ErrNoHost) {
			return err
		}
	}
	return nil
}

// callProfile sends req to the host of profile among hs.
func callProfile(hs []client.Host, profile string, req proto.Request, out any) error {
	h, err := hostFor(hs, profile)
	if err != nil {
		return err
	}
	if err := client.Call(h.Path, req, out); !errors.Is(err, client.ErrNoHost) {
		return err
	}
	return noHostForError{profile}
}

func tabCommand(cmd string, args []string) error {
	f := newFlags(cmd, false)
	if err := f.parse(args); err != nil {
		return err
	}
	if f.NArg() != 1 {
		return badUsage("%s takes exactly one tab id", cmd)
	}
	profile, id, err := parseID("tab", f.Arg(0))
	if err != nil {
		return err
	}
	hs, err := hosts()
	if err != nil {
		return err
	}
	return callProfile(hs, profile, proto.Request{Cmd: cmd, Tab: id}, nil)
}

// hostAnswer is one host's result to a request every host was asked.
type hostAnswer[T any] struct {
	host   client.Host
	result T
}

// askAll sends req to every host at once. A host that is dead, or closes
// the connection because its Chrome is shutting down, is left out; every
// other failure is written to stderr, and the first is returned.
func askAll[T any](hs []client.Host, req proto.Request) (answers []hostAnswer[T], failed error) {
	results := make([]T, len(hs))
	errs := make([]error, len(hs))
	var wg sync.WaitGroup
	for i, h := range hs {
		if h.Conflict != "" {
			errs[i] = errors.New(h.Conflict)
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = client.Call(h.Path, req, &results[i])
		}()
	}
	wg.Wait()
	for i, h := range hs {
		switch err := errs[i]; {
		case err == nil:
			answers = append(answers, hostAnswer[T]{h, results[i]})
		case errors.Is(err, client.ErrNoHost), errors.Is(err, io.EOF):
		default:
			if failed == nil {
				failed = err
			}
			fmt.Fprintf(os.Stderr, "chromectl: %s %s: %v\n", h.Browser, profileLabel(h), err)
		}
	}
	return answers, failed
}

func profileLabel(h client.Host) string {
	if h.Profile == "" {
		return "(old extension)"
	}
	return h.Profile
}

func list(w io.Writer, args []string) error {
	f := newFlags("list", true)
	if err := f.parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return badUsage("list takes no arguments")
	}
	hs, err := hosts()
	if err != nil {
		return err
	}
	answers, failed := askAll[[]proto.Tab](hs, proto.Request{Cmd: "list"})
	if len(answers) == 0 && failed == nil {
		return client.ErrNoHost
	}

	type row struct {
		ID      string `json:"id"`
		Window  string `json:"window"`
		Group   string `json:"group"`
		Active  bool   `json:"active"`
		Browser string `json:"browser"`
		Profile string `json:"profile"`
		Title   string `json:"title"`
		URL     string `json:"url"`
	}
	rows := []row{}
	for _, a := range answers {
		for _, t := range a.result {
			r := row{
				ID:      composite(a.host.Profile, t.ID),
				Window:  composite(a.host.Profile, t.Window),
				Active:  t.Active,
				Browser: a.host.Browser,
				Profile: a.host.Profile,
				Title:   t.Title,
				URL:     t.URL,
			}
			if t.Group > 0 {
				r.Group = composite(a.host.Profile, t.Group)
			}
			rows = append(rows, r)
		}
	}
	if *f.json {
		err = printJSON(w, rows)
	} else {
		for _, r := range rows {
			active, group := "-", r.Group
			if r.Active {
				active = "*"
			}
			if group == "" {
				group = "-"
			}
			if _, err = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.Window, group, active, r.Browser, r.Title, r.URL); err != nil {
				break
			}
		}
	}
	if err == nil && failed != nil {
		err = incompleteError{"list", failed}
	}
	return err
}

func groups(w io.Writer, args []string) error {
	f := newFlags("groups", true)
	if err := f.parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return badUsage("groups takes no arguments")
	}
	hs, err := hosts()
	if err != nil {
		return err
	}
	answers, failed := askAll[[]proto.Group](hs, proto.Request{Cmd: "groups"})
	if len(answers) == 0 && failed == nil {
		return client.ErrNoHost
	}

	type row struct {
		ID     string `json:"id"`
		Window string `json:"window"`
		Title  string `json:"title"`
	}
	rows := []row{}
	for _, a := range answers {
		for _, g := range a.result {
			rows = append(rows, row{composite(a.host.Profile, g.ID), composite(a.host.Profile, g.Window), g.Title})
		}
	}
	if *f.json {
		err = printJSON(w, rows)
	} else {
		for _, r := range rows {
			if _, err = fmt.Fprintf(w, "%s\t%s\t%s\n", r.ID, r.Window, r.Title); err != nil {
				break
			}
		}
	}
	if err == nil && failed != nil {
		err = incompleteError{"groups list", failed}
	}
	return err
}

// activateGroup raises the group titled title in the profile that used it
// last, or creates it with url: in the window's profile where a window id is
// given, else in the one profile that runs.
func activateGroup(w io.Writer, args []string) error {
	f := newFlags("activate-group", true)
	if err := f.parse(args); err != nil {
		return err
	}
	if f.NArg() < 2 || f.NArg() > 3 {
		return badUsage("activate-group takes a title, a url and an optional window id")
	}
	req := proto.Request{Cmd: "activate-group", Title: f.Arg(0), URL: f.Arg(1)}
	if req.Title == "" {
		return badUsage("activate-group needs a title that is not empty")
	}
	var windowProfile string
	if f.NArg() == 3 {
		p, window, err := parseID("window", f.Arg(2))
		if err != nil {
			return err
		}
		windowProfile, req.Window = p, window
	}
	hs, err := hosts()
	if err != nil {
		return err
	}

	answers, failed := askAll[[]proto.Group](hs, proto.Request{Cmd: "groups"})
	profile, latest := "", -1.0
	for _, a := range answers {
		for _, g := range a.result {
			if g.Title == req.Title && g.LastAccessed > latest {
				profile, latest = a.host.Profile, g.LastAccessed
			}
		}
	}
	switch {
	case profile != "" && profile != windowProfile:
		// The group is found; the window is only where a new one would go.
		req.Window = 0
	case profile != "":
	case failed != nil:
		// The host that did not answer may hold the group, so none is created.
		return incompleteError{"group search", failed}
	case windowProfile != "":
		profile = windowProfile
	default:
		if profile, err = onlyProfile(hs, "activate-group"); err != nil {
			return err
		}
	}

	var raised proto.Raised
	if err := callProfile(hs, profile, req, &raised); err != nil {
		return err
	}
	id, window := composite(profile, raised.ID), composite(profile, raised.Window)
	if *f.json {
		return printJSON(w, struct {
			ID      string `json:"id"`
			Window  string `json:"window"`
			Created bool   `json:"created"`
			Title   string `json:"title"`
		}{id, window, raised.Created, raised.Title})
	}
	_, err = fmt.Fprintf(w, "%s\t%s\t%t\t%s\n", id, window, raised.Created, raised.Title)
	return err
}

func open(w io.Writer, args []string) error {
	f := newFlags("open", true)
	if err := f.parse(args); err != nil {
		return err
	}
	if f.NArg() < 1 || f.NArg() > 2 {
		return badUsage("open takes a url and an optional window id")
	}
	req := proto.Request{Cmd: "open", URL: f.Arg(0)}
	var profile string
	if f.NArg() == 2 {
		p, window, err := parseID("window", f.Arg(1))
		if err != nil {
			return err
		}
		profile, req.Window = p, window
	}
	hs, err := hosts()
	if err != nil {
		return err
	}
	if profile == "" {
		if profile, err = onlyProfile(hs, "open"); err != nil {
			return err
		}
	}
	var opened proto.Tab
	if err := callProfile(hs, profile, req, &opened); err != nil {
		return err
	}
	id, window := composite(profile, opened.ID), composite(profile, opened.Window)
	if *f.json {
		return printJSON(w, struct {
			ID     string `json:"id"`
			Window string `json:"window"`
		}{id, window})
	}
	_, err = fmt.Fprintf(w, "%s\t%s\n", id, window)
	return err
}

// onlyProfile returns the profile cmd uses without a window id: the one that
// runs. With several, it is bad usage, and the error lists a window id of
// each, so the caller can choose. It asks each for its tabs to learn a window
// id; that costs a round trip only on the way to an error.
func onlyProfile(hs []client.Host, cmd string) (string, error) {
	var live []client.Host
	for _, h := range hs {
		if h.Profile != "" && h.Conflict == "" {
			live = append(live, h)
		}
	}
	if len(live) == 1 {
		return live[0].Profile, nil
	}
	if len(live) == 0 {
		for _, h := range hs {
			if h.Conflict != "" {
				return "", errors.New(h.Conflict)
			}
		}
		if err := staleReason(hs); err != nil {
			return "", err
		}
		return "", client.ErrNoHost
	}
	answers, failed := askAll[[]proto.Tab](live, proto.Request{Cmd: "list"})
	if len(answers) == 1 && failed == nil {
		return answers[0].host.Profile, nil
	}
	if len(answers) == 0 && failed == nil {
		return "", client.ErrNoHost
	}
	var windows, windowless []string
	for _, a := range answers {
		if len(a.result) > 0 {
			windows = append(windows, fmt.Sprintf("%s (%s)", composite(a.host.Profile, a.result[0].Window), a.host.Browser))
		} else {
			windowless = append(windowless, fmt.Sprintf("%s (%s)", a.host.Profile, a.host.Browser))
		}
	}
	msg := "several Chrome profiles run, so " + cmd + " needs a window id"
	if len(windows) > 0 {
		msg += ", one of " + strings.Join(windows, ", ")
	}
	if len(windowless) > 0 {
		msg += "; without a window, and so out of reach of " + cmd + ": " + strings.Join(windowless, ", ")
	}
	return "", badUsage("%s", msg)
}

// printJSON writes v as one line of JSON. URLs keep their & < > as is.
func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
