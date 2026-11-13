package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Écran virtuel du bureau d'administration (labwc headless). noVNC le règle sur
// sa fenêtre ; Sunshine ne le change jamais lui-même.
const ecranHeadless = "HEADLESS-1"

// motifMode : WIDTHxHEIGHT, avec une fréquence optionnelle (@60).
var motifMode = regexp.MustCompile(`^[0-9]+x[0-9]+(@[0-9]+)?$`)

const usageResolution = `Usage : aibox resolution [1920x1080[@60] | client]
  aibox resolution 1920x1080[@60]   taille choisie
  aibox resolution client           celle du client Moonlight (commande « do » d'une
                                    appli Sunshine : SUNSHINE_CLIENT_*)
  aibox resolution                  taille actuelle`

// cmdResolution : « aibox resolution », taille de l'écran virtuel.
func cmdResolution(args []string, sortie, erreurs io.Writer) int {
	return resolution(envReel(), args, sortie, erreurs)
}

// resolution lit ou change la taille de HEADLESS-1 par wlr-randr, sur wayland-0.
func resolution(e env, args []string, sortie, erreurs io.Writer) int {
	vars := []string{"WAYLAND_DISPLAY=wayland-0"}
	if len(args) == 0 {
		txt, err := e.lancer(vars, "wlr-randr", "--output", ecranHeadless)
		if err != nil {
			fmt.Fprintf(erreurs, "aibox resolution : wlr-randr a échoué : %v\n", err)
			return 1
		}
		for _, l := range strings.Split(txt, "\n") {
			if strings.Contains(l, "current") {
				if champs := strings.Fields(l); len(champs) > 0 {
					fmt.Fprintln(sortie, champs[0])
				}
			}
		}
		return 0
	}

	var mode string
	switch {
	case args[0] == "client":
		mode = valeurDefaut("SUNSHINE_CLIENT_WIDTH", "1920") + "x" + valeurDefaut("SUNSHINE_CLIENT_HEIGHT", "1080")
		if fps := os.Getenv("SUNSHINE_CLIENT_FPS"); fps != "" {
			mode += "@" + fps
		}
	case motifMode.MatchString(args[0]):
		mode = args[0]
	default:
		fmt.Fprintln(erreurs, usageResolution)
		return 1
	}
	return e.executer(vars, "wlr-randr", "--output", ecranHeadless, "--custom-mode", mode)
}

// valeurDefaut : la variable d'environnement, ou la valeur par défaut si elle est vide.
func valeurDefaut(nom, defaut string) string {
	if v := os.Getenv(nom); v != "" {
		return v
	}
	return defaut
}
