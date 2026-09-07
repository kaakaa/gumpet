// Command gumpetctl sends a message to a running gumpet, and opens its
// settings page.
//
//	gumpetctl おなかすいた
//	echo "build finished" | gumpetctl -
//	gumpetctl -d 30 "the deploy is done"
//	gumpetctl -settings
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kaakaa/gumpet/internal/browser"
	"github.com/kaakaa/gumpet/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "gumpetctl: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	defaultPath, _ := config.DefaultPath()
	configPath := flag.String("config", defaultPath, "config file to read the address and token from")
	addr := flag.String("addr", "", "gumpet address, overriding the config file")
	token := flag.String("token", "", "auth token, overriding the config file")
	duration := flag.Float64("d", 0, "seconds to keep the message up (0 uses gumpet's own setting)")
	settings := flag.Bool("settings", false, "open the settings page in a browser")
	flag.Usage = usage
	flag.Parse()

	var text string
	if !*settings {
		var err error
		if text, err = readText(flag.Args()); err != nil {
			return err
		}
		if strings.TrimSpace(text) == "" {
			flag.Usage()
			return fmt.Errorf("no message to send")
		}
	}

	// Reading the config means the common case needs no flags at all.
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if *addr == "" {
		*addr = cfg.Server.Addr
	}
	if *token == "" {
		*token = cfg.Server.Token
	}

	if *settings {
		return openSettings(*addr)
	}
	return send(*addr, *token, text, *duration)
}

// openSettings points a browser at the running gumpet. The URL is printed
// either way, so a failure to launch anything is still useful.
func openSettings(addr string) error {
	url := "http://" + addr + "/"
	fmt.Println(url)
	if err := browser.Open(url); err != nil {
		return fmt.Errorf("open a browser: %w (open the address above yourself)", err)
	}
	return nil
}

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(),
		"usage: gumpetctl [flags] <message>\n"+
			"       gumpetctl [flags] -          (read the message from stdin)\n"+
			"       gumpetctl -settings          (open the settings page)\n\n")
	flag.PrintDefaults()
}

// readText takes the message from the arguments, or from stdin when the only
// argument is "-".
func readText(args []string) (string, error) {
	if len(args) == 1 && args[0] == "-" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return strings.TrimRight(string(b), "\r\n"), nil
	}
	return strings.Join(args, " "), nil
}

func send(addr, token, text string, durationSec float64) error {
	body, err := json.Marshal(struct {
		Text        string  `json:"text"`
		DurationSec float64 `json:"duration_sec,omitempty"`
	}{Text: text, DurationSec: durationSec})
	if err != nil {
		return err
	}

	url := "http://" + addr + "/api/v1/messages"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Gumpet-Token", token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send to %s: %w (is gumpet running?)", addr, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(msg)))
	}
	return nil
}
