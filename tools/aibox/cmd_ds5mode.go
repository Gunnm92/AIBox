package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// modesDs5 : les modes de la manette DualSense.
// natif : haptiques natives de ds5-usbip, loopback arrêté (défaut).
// son   : vibration tirée du son du PC, loopback ds5-haptics-loopback actif.
var modesDs5 = []string{"natif", "son"}

const usageDs5Mode = `Usage : aibox ds5-mode [natif|son]
  natif : haptiques natives de ds5-usbip, loopback arrêté (par défaut)
  son   : vibration tirée du son du PC (service ds5-haptics-loopback)
  aibox ds5-mode                            mode actuel`

// cmdDs5Mode : « aibox ds5-mode », mode de la manette DualSense.
func cmdDs5Mode(args []string, sortie, erreurs io.Writer) int {
	return ds5Mode(envReel(), args, sortie, erreurs)
}

// ds5Mode écrit le mode choisi (dans ~/.config/aibox/ds5-mode), puis arrête ou
// lance le loopback ds5-haptics-loopback par systemd (utilisateur).
func ds5Mode(e env, args []string, sortie, erreurs io.Writer) int {
	fichier := filepath.Join(e.home, ".config", "aibox", "ds5-mode")

	if len(args) == 0 {
		// Mode actuel : celui du fichier, sinon natif.
		if b, err := os.ReadFile(fichier); err == nil {
			fmt.Fprintln(sortie, strings.TrimSpace(string(b)))
			return 0
		}
		fmt.Fprintln(sortie, "natif")
		return 0
	}

	mode := args[0]
	if len(args) > 1 || !contient(modesDs5, mode) {
		fmt.Fprintln(erreurs, usageDs5Mode)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(fichier), 0o755); err != nil {
		fmt.Fprintf(erreurs, "aibox ds5-mode : %v\n", err)
		return 1
	}
	if err := os.WriteFile(fichier, []byte(mode+"\n"), 0o644); err != nil {
		fmt.Fprintf(erreurs, "aibox ds5-mode : %v\n", err)
		return 1
	}
	if mode == "son" {
		fmt.Fprintln(sortie, "Manette DualSense : son (vibration tirée du son du PC, service ds5-haptics-loopback)")
		fmt.Fprintln(sortie, "Dans DS5Dongle-Studio, régler le dongle sur « Replace ».")
		return e.executer(nil, "systemctl", "--user", "start", "ds5-haptics-loopback")
	}
	fmt.Fprintln(sortie, "Manette DualSense : natif (haptiques natives de ds5-usbip, loopback arrêté)")
	fmt.Fprintln(sortie, "Dans DS5Dongle-Studio, régler le dongle sur « Off ».")
	return e.executer(nil, "systemctl", "--user", "stop", "ds5-haptics-loopback")
}
