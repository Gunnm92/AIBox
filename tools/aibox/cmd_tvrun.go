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
  Lance une commande sur l'écran de la TV (Plasma Bigscreen). La TV doit être en
  mode bigscreen : aibox tv-mode --once bigscreen. Depuis le bureau
  d'administration (Selkies), pour un cas que Bigscreen ne gère pas, par exemple :
    aibox tv-run env WSQUASHFS_HIDRAW=1 wsquashfs-launcher "<jeu>.wsquashfs"`

// cmdTvRun : « aibox tv-run », lance une commande sur l'écran de la TV.
func cmdTvRun(args []string, sortie, erreurs io.Writer) int {
	return tvRun(envReel(), args, erreurs, remplacerProcessus)
}

// remplacerProcessus remplace le processus courant par la commande (comme exec).
func remplacerProcessus(chemin string, argv []string, vars []string) error {
	return syscall.Exec(chemin, argv, append(os.Environ(), vars...))
}

// tvRun lit l'affichage de la TV (aibox-tv.env, écrit par session-env au démarrage
// de sa session Plasma), vérifie que son socket écoute, puis remplace le
// processus par la commande, avec WAYLAND_DISPLAY, DISPLAY et XAUTHORITY de la TV.
func tvRun(e env, args []string, erreurs io.Writer, remplacer func(chemin string, argv []string, vars []string) error) int {
	tv := lireEnv(filepath.Join(e.runtime, "aibox-tv.env"))
	wayland := tv["WAYLAND_DISPLAY"]
	if wayland == "" || !estSocket(filepath.Join(e.runtime, wayland)) {
		fmt.Fprintln(erreurs, "La TV n'est pas sous Plasma Bigscreen. Pour y passer : aibox tv-mode --once bigscreen")
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
	vars := []string{"WAYLAND_DISPLAY=" + wayland}
	for _, v := range []string{"DISPLAY", "XAUTHORITY"} {
		if tv[v] != "" {
			vars = append(vars, v+"="+tv[v])
		}
	}
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
