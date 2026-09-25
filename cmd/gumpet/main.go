// Command gumpet runs the desktop pet and the HTTP endpoint that talks to it.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/kaakaa/gumpet/internal/config"
	"github.com/kaakaa/gumpet/internal/feed"
	"github.com/kaakaa/gumpet/internal/history"
	"github.com/kaakaa/gumpet/internal/message"
	"github.com/kaakaa/gumpet/internal/pet"
	"github.com/kaakaa/gumpet/internal/petpack"
	"github.com/kaakaa/gumpet/internal/server"
	"github.com/kaakaa/gumpet/internal/settings"
	"github.com/kaakaa/gumpet/internal/update"
)

// version is stamped in by the build; see the Makefile.
var version = "dev"

// ticksPerSecond is deliberately below the usual 60: the pet is a resident
// application, and its animation runs at a handful of frames a second anyway.
const ticksPerSecond = 30

// inboxSize is how many messages may sit between the HTTP handler and the game
// loop, which drains the channel every tick.
const inboxSize = 64

// restartRequested is set when gumpet should start again once it has stopped:
// after an update is installed, or when the menu's Restart is picked. The
// restart itself waits until run has returned — the pet's window closed and,
// above all, the server let go of its address, which the new process has to
// take.
var restartRequested atomic.Bool

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gumpet: %v\n", err)
		os.Exit(1)
	}
	if restartRequested.Load() {
		exe, err := os.Executable()
		if err == nil {
			err = update.Restart(exe)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "gumpet: could not start again: %v\n", err)
			os.Exit(1)
		}
	}
}

func run() error {
	defaultPath, pathErr := config.DefaultPath()
	configPath := flag.String("config", defaultPath, "path to the config file")
	printConfig := flag.Bool("print-config", false, "print the default config file and exit")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()

	switch {
	case *showVersion:
		fmt.Println("gumpet", version)
		return nil
	case *printConfig:
		fmt.Print(config.DefaultYAML())
		return nil
	}
	if *configPath == "" {
		return pathErr
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	log.Info("loaded config", "path", *configPath)

	pack, err := petpack.Load(cfg.Pet.Source)
	if err != nil {
		return err
	}
	log.Info("loaded pet", "pack", pack.Name, "frames", len(pack.Walk), "size", pack.Size)
	if cfg.Window.ClickThrough {
		// Easy to end up with and impossible to work out from the outside: the
		// window gets no mouse input at all, so the pet cannot be clicked.
		log.Warn("window.click_through is on, so clicking the pet will not open its menu",
			"fix", "set window.click_through to false, or run: gumpetctl -settings")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store := settings.New(cfg, *configPath)
	hist := history.New(cfg.History)
	remarks := history.NewRemarks(cfg.History)
	feedReports := feed.NewReports()
	inbox := make(chan message.Message, inboxSize)
	monitors := pet.Monitors()
	srv := server.New(store, hist, monitors, inbox, log)
	srv.SetRemarks(remarks)
	srv.SetFeedReports(feedReports)
	// Restarting ends the context, which ends both the game loop and the
	// server the same way Ctrl-C does; main starts gumpet again afterwards.
	restart := func() {
		restartRequested.Store(true)
		stop()
	}
	if exe, err := os.Executable(); err != nil {
		log.Warn("updates are off: cannot tell where this gumpet is", "error", err)
	} else {
		// What an update on Windows had to leave behind: the old executable,
		// which could not be deleted while it was running.
		update.Cleanup(exe)
		srv.SetUpdater(update.New(version, exe), restart)
	}

	// Claim the port before opening a window. gumpet exists to be sent
	// messages, so one that cannot listen has nothing to offer: starting
	// anyway leaves a pet walking about that looks entirely normal and quietly
	// receives nothing.
	ln, err := srv.Listen()
	if err != nil {
		return fmt.Errorf("%w\n\nis gumpet already running? only one can hold an address, "+
			"and the second one would not be able to receive anything", err)
	}

	served := make(chan struct{})
	go func() {
		defer close(served)
		// Reported here rather than after the game ends, so that a server that
		// stops during a session is news at the time.
		if err := srv.Serve(ctx, ln); err != nil {
			log.Error("message server stopped", "error", err)
		}
	}()

	for _, m := range monitors {
		log.Info("monitor", "display", m.Label())
	}

	applyWindowSettings(cfg)
	game := pet.New(pet.Options{
		Store:       store,
		Pack:        pack,
		Inbox:       inbox,
		History:     hist,
		Remarks:     remarks,
		FeedReports: feedReports,
		Quit:        ctx.Done(),
		Restart:     restart,
		Log:         log,
		Version:     version,
	})

	// Ebitengine has to own the main goroutine, and returns once the window is
	// closed or Update reports ebiten.Termination.
	if err := ebiten.RunGameWithOptions(game, &ebiten.RunGameOptions{
		ScreenTransparent: true,
		SkipTaskbar:       cfg.Window.SkipTaskbar,
		InitUnfocused:     true,
	}); err != nil {
		return err
	}

	stop()
	<-served
	return nil
}

// applyWindowSettings turns the window into an undecorated, transparent
// overlay. Its size and position are the pet's business, and are set on every
// tick as the pet moves.
func applyWindowSettings(cfg config.Config) {
	pet.UseDisplay(cfg.Stage.Display)
	ebiten.SetWindowTitle("gumpet")
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeDisabled)
	ebiten.SetWindowFloating(cfg.Window.AlwaysOnTop)
	ebiten.SetWindowMousePassthrough(cfg.Window.ClickThrough)
	ebiten.SetTPS(ticksPerSecond)
}
