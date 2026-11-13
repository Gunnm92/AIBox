package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// modesTv : les modes de la session TV.
var modesTv = []string{"gamescope", "hyprland", "bureau"}

const usageTvMode = `Usage : aibox tv-mode [--once] [gamescope|hyprland|bureau]
  gamescope : Steam Big Picture sous gamescope (HDR, FSR)
  hyprland  : Hyprland, cartes de widgets et lanceur à tuiles (par défaut)
  bureau    : labwc et le bureau XFCE, sans le lanceur ; Super+L revient au lanceur
  --once    : pour la prochaine session seulement (en RAM, oublié ensuite)
  aibox tv-mode                             mode actuel`

// cmdTvMode : « aibox tv-mode », mode de la session TV, relance la session.
func cmdTvMode(args []string, sortie, erreurs io.Writer) int {
	return tvMode(envReel(), args, sortie, erreurs)
}

// tvMode écrit le mode choisi (dans ~/.config/aibox/tv-mode, ou en RAM avec
// --once), puis relance la session TV par systemd.
func tvMode(e env, args []string, sortie, erreurs io.Writer) int {
	gardé := filepath.Join(e.home, ".config", "aibox", "tv-mode")
	pourUneSession := filepath.Join(e.runtime, "aibox-tv-mode-once")
	conf := gardé
	if len(args) > 0 && args[0] == "--once" {
		conf = pourUneSession
		args = args[1:]
	}

	if len(args) == 0 {
		// Mode actuel : celui de cette session, sinon le gardé, sinon hyprland.
		for _, f := range []string{pourUneSession, gardé} {
			if b, err := os.ReadFile(f); err == nil {
				fmt.Fprintln(sortie, strings.TrimSpace(string(b)))
				return 0
			}
		}
		fmt.Fprintln(sortie, "hyprland")
		return 0
	}

	mode := args[0]
	if len(args) > 1 || !contient(modesTv, mode) {
		fmt.Fprintln(erreurs, usageTvMode)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
		fmt.Fprintf(erreurs, "aibox tv-mode : %v\n", err)
		return 1
	}
	if err := os.WriteFile(conf, []byte(mode+"\n"), 0o644); err != nil {
		fmt.Fprintf(erreurs, "aibox tv-mode : %v\n", err)
		return 1
	}
	if conf == pourUneSession {
		fmt.Fprintf(sortie, "Session TV : %s, pour cette session (redémarrage de la session)\n", mode)
	} else {
		fmt.Fprintf(sortie, "Session TV : %s (redémarrage de la session)\n", mode)
	}
	return e.executer(nil, "sudo", "systemctl", "restart", "aibox-tv")
}

// contient : vrai si la liste contient la valeur.
func contient(liste []string, v string) bool {
	for _, x := range liste {
		if x == v {
			return true
		}
	}
	return false
}
