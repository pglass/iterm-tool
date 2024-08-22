package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pglass/iterm-tool/config"
	"github.com/pglass/iterm-tool/iterm2"
)

var (
	flagConfigFile string

	sessionProps = iterm2.CustomProfileProperties{
		TitleComponents: iterm2.TitleComponentSessionName,
	}
)

func init() {
	flag.StringVar(&flagConfigFile, "c", "", "config file")
}

func main() {
	flag.Parse()

	if len(flagConfigFile) == 0 {
		log.Fatalf("config file is required")
	}

	slog.SetLogLoggerLevel(slog.LevelDebug)
	slog.Info("load config", "file", flagConfigFile)

	cfg, err := config.LoadConfig(flagConfigFile)
	die("load config", err)

	cache, err := NewCache()
	die("init cache", err)

	slog.Info("creating stack", "id", cfg.ID)
	app, err := iterm2.NewApp(cfg.ID)
	if err != nil {
		log.Fatal(err)
	}
	defer app.Close()

	windows, err := app.ListWindows()
	die("list windows", err)

	cached, err := cache.Get(cfg.ID)
	die("read cache", err)

	// Find the existing window or create new window.
	for _, w := range windows {
		if w.ID() == cached.WindowID {
			slog.Info("closing existing window", "id", w.ID())
			die("closing existing window", w.Close(true))
			break
		}
	}

	window, err := app.CreateWindow(&iterm2.CreateWindowOpts{
		CustomProfileProperties: sessionProps,
	})
	die("create window", err)

	cached.WindowID = window.ID()
	die("write cache", cache.Put(cfg.ID, cached))

	slog.Info("created window", "id", window.ID())

	err = window.SetTitle(cfg.ID)
	die("set window title", err)

	tabs, err := window.ListTabs()
	die("list tabs", err)
	if len(tabs) == 0 {
		log.Fatal("no tabs in window")
	}

	tab := tabs[0]
	sessions, err := tab.ListSessions()
	die("list sessions", err)
	if len(sessions) == 0 {
		log.Fatalf("no sessions in tab")
	}

	assignment := map[string]iterm2.Session{}
	lastInGroup := map[string]iterm2.Session{}

	addSplit := func(sessCfg *config.Session, splitFrom iterm2.Session, vertical bool) {
		group := sessCfg.Group()
		if len(assignment) == 0 {
			// Tabs are created with a split. Assign it on the first call.
			assignment[sessCfg.Name] = sessions[0]
			lastInGroup[group] = sessions[0]
			slog.Info("assigned initial session", "name", sessCfg.Name, "group", group)
			return
		}

		sess, err := splitFrom.SplitPane(iterm2.SplitPaneOptions{
			Vertical:                vertical,
			CustomProfileProperties: sessionProps,
		})
		die("split pane", err)

		assignment[sessCfg.Name] = sess
		lastInGroup[group] = sess
		sessions = append(sessions, sess)
		slog.Info("assigned new session", "name", sessCfg.Name, "group", group)
	}

	// Use a simple rule for splits:
	// - Do all vertical splits before horizontal splits.
	// - Do one vsplit per group (so, one column per group)
	// - Do one hsplit for additional group members.
	sessionConfigsByGroup := cfg.SessionsByGroup()
	for _, group := range SortedKeys(sessionConfigsByGroup) {
		// Create one vertical split per group
		// We'll assign the first session in each group to a vpslit.
		sessCfg := sessionConfigsByGroup[group][0]
		splitFrom := sessions[len(sessions)-1]
		addSplit(sessCfg, splitFrom, true /* vertical */)
	}
	for group, cfgs := range cfg.SessionsByGroup() {
		// We already assigned the first cfg in each group.
		for _, sessCfg := range cfgs[1:] {
			splitFrom := lastInGroup[group]
			addSplit(sessCfg, splitFrom, false /* not vertical */)
		}
	}

	// Prep sessions.
	// - Navigate to a specified directory.
	for name := range cfg.Sessions {
		sess, ok := assignment[name]
		if !ok {
			log.Fatalf("[bug] no assigned session: name=%s", name)
		}
		die("set session name", sess.SetName(name))
		if cfg.Directory != "" {
			die("send text", sess.SendText(fmt.Sprintf("cd %s\n", cfg.Directory)))
		}
	}

	// We need to traverse a dependency tree of session config.
	// I'm lazy, so the way this will work is:
	//
	//  0. Run commands for sessions with no dependencies
	//  1. Find the next group of sessions that can run.
	//     - Iterate all sessions. If the session is not done, and all
	//       items in a session's depends_on are done, then we'll
	//       process this session config next.
	//  2. If not all done, repeat from 1
	//
	// We just need to maintain a map of "done" sessions.
	doneSessions := map[string]struct{}{}
	for len(doneSessions) < len(cfg.Sessions) {

		var wg sync.WaitGroup

		for _, scfg := range getNextSessionsInTree(doneSessions, cfg) {
			scfg := scfg

			sess, ok := assignment[scfg.Name]
			if !ok {
				log.Fatalf("[bug] no assigned session: name=%s", scfg.Name)
			}

			// TODO: context
			wg.Add(1)
			go func() {
				defer wg.Done()
				if scfg.Script != "" {
					if err := feedScriptAndWaitForDone(sess, scfg); err != nil {
						slog.Error("running script", "error", err)
					}
				}
				if scfg.Inject != "" {
					if err := feedInject(sess, scfg); err != nil {
						slog.Error("running inject", "error", err)
					}
				}
				doneSessions[scfg.Name] = struct{}{}
			}()
		}

		// Wait for this group to finish.
		// TODO: Some scripts never end. We need some way to start those, but not wait for them.
		wg.Wait()
	}
}

func feedScriptAndWaitForDone(session iterm2.Session, scfg *config.Session) error {
	doneFile, err := os.CreateTemp("", fmt.Sprintf("%s-done-*", scfg.Name))
	die("create temp file", err)
	defer os.Remove(doneFile.Name())
	doneFile.Close()

	scriptFile, err := os.CreateTemp("", fmt.Sprintf("%s-script-*", scfg.Name))
	die("create temp file", err)
	defer os.Remove(scriptFile.Name())

	slog.Info("preparing session files", "done", doneFile.Name(), "script", scriptFile.Name())

	// Run the configured script.
	scriptFile.WriteString("set -x\n")
	scriptFile.WriteString(scfg.Script)

	// After the configured script is done.
	scriptFile.WriteString(fmt.Sprintf("\necho 'done' > %s\n", doneFile.Name()))
	scriptFile.Close()

	time.Sleep(1 * time.Second)

	die("send text", session.SendText(
		fmt.Sprintf("bash %s\n", scriptFile.Name())),
	)

	slog.Info("started sesssion", "name", scfg.Name)

	// Wait for the pid file to be written
	for {
		data, err := os.ReadFile(doneFile.Name())
		if err != nil {
			return fmt.Errorf("unable to read done file: %w", err)
		}
		content := strings.TrimSpace(string(data))
		if len(content) != 0 {
			break
		}

		time.Sleep(2 * time.Second)

		// Check if the session has closed.
		if _, err := session.GetVariable("jobName"); err != nil {
			return fmt.Errorf("session closed while waiting for script (unable to get variable jobName from session, which is the hack I'm using to check if closed: %w)", err)
		}
	}
	return nil
}

func feedInject(session iterm2.Session, scfg *config.Session) error {
	slog.Info("feeding inject lines", "session", scfg.Name)

	die("send text", session.SendText(scfg.Inject+"\n"))

	// TODO: How to check we're done? I don't want to modify the inject lines
	// because I want to up arrow easily.

	return nil
}

func getNextSessionsInTree(doneSessions map[string]struct{}, cfg *config.Config) []*config.Session {
	isDone := func(name string) bool {
		_, done := doneSessions[name]
		return done
	}

	var result []*config.Session
	for _, scfg := range cfg.Sessions {
		if isDone(scfg.Name) {
			continue
		}

		dependsOnDone := true
		for _, name := range scfg.DependsOn {
			if strings.HasPrefix(name, "sessions.") {
				name = strings.SplitN(name, ".", 2)[1]
			}
			if !isDone(name) {
				dependsOnDone = false
			}
		}

		if dependsOnDone {
			result = append(result, scfg)
		}
	}

	return result
}

func die(msg string, err error) {
	if err != nil {
		log.Fatalf("%s error: %s", msg, err)
	}
}

func SortedKeys[V any](m map[string]V) []string {
	result := []string{}
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
