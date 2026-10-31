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

var compositeID = regexp.MustCompile(`^([0-9a-f]{8}):([0-9]+)$`)

// parseID reads a tab or window id as list prints it: the profile that owns
// it, a colon, and Chrome's number, which starts at 1.
func parseID(kind, arg string) (profile string, id int, err error) {
	m := compositeID.FindStringSubmatch(arg)
	if m != nil {
		id, err = strconv.Atoi(m[2])
	}
	if m == nil || err != nil || id < 1 {
		return "", 0, fmt.Errorf("%s id %q is not of the form <profile>:<number> that list prints", kind, arg)
	}
	return m[1], id, nil
}

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
	for _, h := range hs {
		if h.Profile == "" {
			return client.Host{}, errors.New(proto.StaleExtension)
		}
	}
	return client.Host{}, fmt.Errorf("no chromectl host for profile %s: is that profile still open?", profile)
}

// callProfile sends req to the host of profile.
func callProfile(profile string, req proto.Request, out any) error {
	hs, err := hosts()
	if err != nil {
		return err
	}
	h, err := hostFor(hs, profile)
	if err != nil {
		return err
	}
	if err := client.Call(h.Path, req, out); !errors.Is(err, client.ErrNoHost) {
		return err
	}
	return fmt.Errorf("no chromectl host for profile %s: is that profile still open?", profile)
}

func tabCommand(cmd string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("%s takes exactly one tab id", cmd)
	}
	profile, id, err := parseID("tab", args[0])
	if err != nil {
		return err
	}
	return callProfile(profile, proto.Request{Cmd: cmd, Tab: id}, nil)
}

// hostTabs is one host's answer to list.
type hostTabs struct {
	host client.Host
	tabs []proto.Tab
}

// listAll asks every host for its tabs at once. A host that is dead or
// shutting down is left out; every other failure is written to stderr, and
// makes complete false.
func listAll(hs []client.Host) (answers []hostTabs, complete bool) {
	tabs := make([][]proto.Tab, len(hs))
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
			errs[i] = client.Call(h.Path, proto.Request{Cmd: "list"}, &tabs[i])
		}()
	}
	wg.Wait()
	complete = true
	for i, h := range hs {
		switch err := errs[i]; {
		case err == nil:
			answers = append(answers, hostTabs{h, tabs[i]})
		case errors.Is(err, client.ErrNoHost), strings.HasPrefix(err.Error(), proto.ChromeGone):
		default:
			complete = false
			fmt.Fprintf(os.Stderr, "chromectl: %s %s: %v\n", h.Browser, profileLabel(h), err)
		}
	}
	return answers, complete
}

func profileLabel(h client.Host) string {
	if h.Profile == "" {
		return "(old extension)"
	}
	return h.Profile
}

func list(w io.Writer, args []string) error {
	fs := newFlagSet("list")
	asJSON := fs.Bool("json", false, "print a JSON array")
	_ = fs.Parse(args)
	if fs.NArg() != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	hs, err := hosts()
	if err != nil {
		return err
	}
	answers, complete := listAll(hs)
	if len(answers) == 0 && complete {
		return client.ErrNoHost
	}

	type row struct {
		ID      string `json:"id"`
		Window  string `json:"window"`
		Active  bool   `json:"active"`
		Browser string `json:"browser"`
		Profile string `json:"profile"`
		Title   string `json:"title"`
		URL     string `json:"url"`
	}
	rows := []row{}
	for _, a := range answers {
		for _, t := range a.tabs {
			rows = append(rows, row{
				ID:      fmt.Sprintf("%s:%d", a.host.Profile, t.ID),
				Window:  fmt.Sprintf("%s:%d", a.host.Profile, t.Window),
				Active:  t.Active,
				Browser: a.host.Browser,
				Profile: a.host.Profile,
				Title:   t.Title,
				URL:     t.URL,
			})
		}
	}
	if *asJSON {
		err = printJSON(w, rows)
	} else {
		for _, r := range rows {
			active := "-"
			if r.Active {
				active = "*"
			}
			_, err = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.ID, r.Window, active, r.Browser, r.Title, r.URL)
		}
	}
	if err == nil && !complete {
		err = errors.New("the list is incomplete: a host above did not answer")
	}
	return err
}

func open(w io.Writer, args []string) error {
	fs := newFlagSet("open")
	asJSON := fs.Bool("json", false, "print a JSON object")
	_ = fs.Parse(args)
	if fs.NArg() < 1 || fs.NArg() > 2 {
		return fmt.Errorf("open takes a url and an optional window id")
	}
	req := proto.Request{Cmd: "open", URL: fs.Arg(0)}
	var profile string
	if fs.NArg() == 2 {
		p, window, err := parseID("window", fs.Arg(1))
		if err != nil {
			return err
		}
		profile, req.Window = p, window
	} else {
		p, err := onlyProfile()
		if err != nil {
			return err
		}
		profile = p
	}
	var opened proto.Tab
	if err := callProfile(profile, req, &opened); err != nil {
		return err
	}
	id, window := fmt.Sprintf("%s:%d", profile, opened.ID), fmt.Sprintf("%s:%d", profile, opened.Window)
	if *asJSON {
		return printJSON(w, struct {
			ID     string `json:"id"`
			Window string `json:"window"`
		}{id, window})
	}
	_, err := fmt.Fprintf(w, "%s\t%s\n", id, window)
	return err
}

// onlyProfile returns the profile open uses without a window id: the one
// that runs. With several, it is an error that lists a window id of each, so
// the caller can choose.
func onlyProfile() (string, error) {
	hs, err := hosts()
	if err != nil {
		return "", err
	}
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
		if len(hs) > 0 {
			return "", errors.New(proto.StaleExtension)
		}
		return "", client.ErrNoHost
	}
	answers, complete := listAll(live)
	if len(answers) == 1 && complete {
		return answers[0].host.Profile, nil
	}
	if len(answers) == 0 && complete {
		return "", client.ErrNoHost
	}
	var windows []string
	for _, a := range answers {
		if len(a.tabs) > 0 {
			windows = append(windows, fmt.Sprintf("%s:%d (%s)", a.host.Profile, a.tabs[0].Window, a.host.Browser))
		}
	}
	return "", fmt.Errorf("several Chrome profiles run: give open a window id, one of %s", strings.Join(windows, ", "))
}

// printJSON writes v as one line of JSON. URLs keep their & < > as is.
func printJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
