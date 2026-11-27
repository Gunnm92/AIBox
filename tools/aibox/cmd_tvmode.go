package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// modesTv : les modes de la session TV.
var modesTv = []string{"gamescope", "bigscreen"}

// anciensModesTv : modes retirés le 27/11/2026 (Hyprland et son lanceur, labwc
// et XFCE), lus dans un ancien ~/.config/aibox/tv-mode ou donnés à la commande :
// ils valent bigscreen (tv-session fait de même).
var anciensModesTv = map[string]string{"hyprland": "bigscreen", "bureau": "bigscreen"}

const usageTvMode = `Usage : aibox tv-mode [--once] [gamescope|bigscreen]
  bigscreen : Plasma Bigscreen, accueil à la manette (par défaut) ; la tuile
              « Bureau » y passe au bureau Plasma et en revient
  gamescope : Steam Big Picture sous gamescope (HDR, FSR)
  --once    : pour la prochaine session seulement (en RAM, oublié ensuite)
  aibox tv-mode                             mode actuel`

// modeTv : le mode, avec les anciens noms ramenés à bigscreen.
func modeTv(mode string) string {
	if nouveau, ok := anciensModesTv[mode]; ok {
		return nouveau
	}
	return mode
}

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
		// Mode actuel : celui de cette session, sinon le gardé, sinon bigscreen.
		for _, f := range []string{pourUneSession, gardé} {
			if b, err := os.ReadFile(f); err == nil {
				fmt.Fprintln(sortie, modeTv(strings.TrimSpace(string(b))))
				return 0
			}
		}
		fmt.Fprintln(sortie, "bigscreen")
		return 0
	}

	mode := modeTv(args[0])
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
