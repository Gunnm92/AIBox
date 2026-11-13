package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const usageTvRun = `Usage : aibox tv-run commande [argument...]
  Lance une commande sur l'écran de la TV (Hyprland, wayland-1 et :1). La TV doit
  être en mode lanceur : aibox tv-mode --once hyprland. Depuis le bureau
  d'administration (noVNC), pour un cas que le lanceur ne gère pas, par exemple :
    aibox tv-run env WSQUASHFS_HIDRAW=1 wsquashfs-launcher "<jeu>.wsquashfs"`

// cmdTvRun : « aibox tv-run », lance une commande sur l'écran de la TV.
func cmdTvRun(args []string, sortie, erreurs io.Writer) int {
	return tvRun(envReel(), args, erreurs, remplacerProcessus)
}

// remplacerProcessus remplace le processus courant par la commande (comme exec).
func remplacerProcessus(chemin string, argv []string, vars []string) error {
	return syscall.Exec(chemin, argv, append(os.Environ(), vars...))
}

// tvRun vérifie que la TV écoute (socket wayland-1), puis remplace le
// processus par la commande, avec WAYLAND_DISPLAY et DISPLAY de la TV.
func tvRun(e env, args []string, erreurs io.Writer, remplacer func(chemin string, argv []string, vars []string) error) int {
	socket := filepath.Join(e.runtime, "wayland-1")
	if !estSocket(socket) {
		fmt.Fprintf(erreurs, "La TV n'est pas en mode lanceur (pas de %s). Pour y passer : aibox tv-mode --once hyprland\n", socket)
		return 1
	}
	if len(args) == 0 {
		fmt.Fprintln(erreurs, usageTvRun)
		return 2
	}
	chemin, err := exec.LookPath(args[0])
	if err != nil {
		fmt.Fprintf(erreurs, "aibox tv-run : commande introuvable : %s\n", args[0])
		return 127
	}
	vars := []string{"WAYLAND_DISPLAY=wayland-1", "DISPLAY=:1"}
	if err := remplacer(chemin, args, vars); err != nil {
		fmt.Fprintf(erreurs, "aibox tv-run : %v\n", err)
		return 126
	}
	return 0
}

// estSocket : vrai si le chemin existe et est un socket Unix.
func estSocket(chemin string) bool {
	info, err := os.Stat(chemin)
	return err == nil && info.Mode()&os.ModeSocket != 0
}
