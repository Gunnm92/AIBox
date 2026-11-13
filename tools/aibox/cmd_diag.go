package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

// dossierDiag : où le rapport est écrit (en RAM), puis copié sur la clé.
const dossierDiag = "run/aibox-diag"

// cmdDiag : « aibox diag », rapport de diagnostic complet. Relancé par sudo hors root.
func cmdDiag(args []string, sortie, erreurs io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(erreurs, "Usage : sudo aibox diag")
		return 2
	}
	if os.Geteuid() != 0 {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintf(erreurs, "aibox diag : %v\n", err)
			return 1
		}
		if err := syscall.Exec("/usr/bin/sudo", []string{"sudo", exe, "diag"}, os.Environ()); err != nil {
			fmt.Fprintf(erreurs, "aibox diag : sudo : %v\n", err)
			return 1
		}
	}
	return diag(envReel(), time.Now(), sortie)
}

// diag écrit le rapport, le copie sur la clé Ventoy ou AIBOXBOOT si elle est là,
// et annonce les deux emplacements.
func diag(e env, maintenant time.Time, sortie io.Writer) int {
	out := e.chemin(dossierDiag)
	if err := os.MkdirAll(out, 0o755); err != nil {
		fmt.Fprintf(sortie, "aibox diag : %v\n", err)
		return 1
	}
	rapport := filepath.Join(out, "aibox-diag-"+maintenant.Format("20060102-150405")+".txt")
	var b bytes.Buffer
	ecrireRapport(e, &b, maintenant)
	if err := os.WriteFile(rapport, b.Bytes(), 0o644); err != nil {
		fmt.Fprintf(sortie, "aibox diag : %v\n", err)
		return 1
	}

	dest := copierRapport(e, rapport, filepath.Base(rapport))
	fmt.Fprintf(sortie, "Rapport : %s\n", rapport)
	if dest != "" {
		fmt.Fprintf(sortie, "Copié sur %s\n", dest)
	} else {
		fmt.Fprintln(sortie, "(ni clé Ventoy ni partition AIBOXBOOT : rapport en RAM seulement)")
	}
	return 0
}

// section : un titre et le contenu.
func section(w io.Writer, titre string) {
	fmt.Fprintf(w, "\n===== %s =====\n", titre)
}

// ecrireRapport rassemble les sections du diagnostic, dans le même ordre que le script.
func ecrireRapport(e env, w io.Writer, maintenant time.Time) {
	cmd := func(nom string, args ...string) string { return e.combine(nil, nom, args...) }

	section(w, "Système")
	fmt.Fprintln(w, maintenant.Format("Mon Jan _2 15:04:05 MST 2006"))
	fmt.Fprint(w, cmd("uname", "-a"))
	if txt, ok := e.lire("proc/cmdline"); ok {
		fmt.Fprintln(w, txt)
	}

	section(w, "Session TV (système)")
	fmt.Fprint(w, premieresLignes(cmd("systemctl", "status", "aibox-tv", "--no-pager"), 25))

	section(w, "Journal aibox-tv")
	fmt.Fprint(w, dernieresLignes(cmd("journalctl", "-b", "-u", "aibox-tv", "--no-pager"), 80))

	section(w, "Sessions logind")
	fmt.Fprint(w, cmd("loginctl", "list-sessions", "--no-pager"))
	fmt.Fprint(w, premieresLignes(cmd("loginctl", "show-seat", "seat0"), 10))

	section(w, "Processus graphiques")
	mode := "hyprland"
	if txt, ok := e.lire("home/default/.config/aibox/tv-mode"); ok && txt != "" {
		mode = txt
	}
	fmt.Fprintf(w, "mode TV : %s\n", mode)
	// aibox-tv-launcher dépasse 15 caractères : pgrep -f cherche sur la ligne de commande.
	for _, recherche := range [][]string{{"-a", "gamescope"}, {"-a", "Hyprland"}, {"-a", "labwc"}, {"-af", "aibox-tv-launcher"}, {"-a", "Xwayland"}} {
		fmt.Fprint(w, cmd("pgrep", recherche...))
	}

	section(w, "Sockets Wayland")
	for _, l := range strings.Split(cmd("ls", "-la", "/run/user/1000/"), "\n") {
		if regexp.MustCompile(`wayland|pipewire`).MatchString(l) {
			fmt.Fprintln(w, l)
		}
	}

	section(w, "Écrans (DRM)")
	for _, c := range e.cartesEcrans() {
		fmt.Fprintf(w, "%s: %s %s\n", c.nom, c.statut, c.premierMode)
	}

	section(w, "Noyau : amdgpu / drm")
	fmt.Fprint(w, filtrer(cmd("journalctl", "-b", "-k", "--no-pager"), `(?i)amdgpu|drm|dcn|dmcub|hdmi`, 60))

	section(w, "Services en échec")
	fmt.Fprint(w, cmd("systemctl", "--failed", "--no-pager"))
	fmt.Fprint(w, e.combine([]string{"XDG_RUNTIME_DIR=/run/user/1000"}, "systemctl", "--user", "--failed", "--no-pager"))

	section(w, "Réseau")
	fmt.Fprint(w, cmd("ip", "-br", "addr"))

	section(w, "Disques")
	fmt.Fprint(w, cmd("lsblk", "-o", "NAME,SIZE,MODEL,FSTYPE,LABEL,MOUNTPOINTS"))

	section(w, "Mémoire")
	afficherMem(e, w)

	section(w, "Journal complet du démarrage")
	fmt.Fprint(w, cmd("journalctl", "-b", "--no-pager"))
}

// ecranDrm : un connecteur d'écran (sysfs), avec son statut et son premier mode.
type ecranDrm struct {
	nom, statut, premierMode string
}

// cartesEcrans : les connecteurs DRM (cardN-*), comme la boucle du script.
func (e env) cartesEcrans() []ecranDrm {
	var res []ecranDrm
	motifs, _ := filepath.Glob(e.chemin("sys/class/drm/card*-*"))
	for _, c := range motifs {
		statut, _ := os.ReadFile(filepath.Join(c, "status"))
		modes, _ := os.ReadFile(filepath.Join(c, "modes"))
		premier := strings.SplitN(string(modes), "\n", 2)[0]
		res = append(res, ecranDrm{
			nom:         filepath.Base(c),
			statut:      strings.TrimSpace(string(statut)),
			premierMode: strings.TrimSpace(premier),
		})
	}
	return res
}

// copierRapport copie le rapport sur la clé Ventoy (données), sinon sur AIBOXBOOT.
// Monte la partition seulement si elle ne l'est pas, et la démonte après.
// Renvoie la destination, ou vide si aucune partition n'est disponible.
func copierRapport(e env, rapport, nom string) string {
	for _, etiquette := range []string{"Ventoy", "AIBOXBOOT"} {
		part := strings.TrimSpace(firstLine(e.combine(nil, "blkid", "-L", etiquette)))
		if part == "" {
			continue
		}
		mnt := ""
		for _, m := range e.montages() {
			if m.source == part {
				mnt = m.point
				break
			}
		}
		propre := false
		if mnt == "" {
			mnt = e.chemin("run/aibox-diag-mnt")
			os.MkdirAll(mnt, 0o755)
			if _, err := e.lancer(nil, "mount", part, mnt); err != nil {
				continue
			}
			propre = true
		}
		dossier := filepath.Join(mnt, "aibox-diag")
		ok := os.MkdirAll(dossier, 0o755) == nil
		if ok {
			b, err := os.ReadFile(rapport)
			ok = err == nil && os.WriteFile(filepath.Join(dossier, nom), b, 0o644) == nil
		}
		if ok {
			syscall.Sync()
		}
		if propre {
			e.lancer(nil, "umount", mnt)
		}
		if ok {
			return etiquette + " : aibox-diag/" + nom
		}
	}
	return ""
}

// premieresLignes : les n premières lignes d'un texte.
func premieresLignes(txt string, n int) string {
	lignes := strings.SplitAfter(txt, "\n")
	if len(lignes) > n {
		lignes = lignes[:n]
	}
	return strings.Join(lignes, "")
}

// dernieresLignes : les n dernières lignes d'un texte, comme tail (fin de ligne conservée).
func dernieresLignes(txt string, n int) string {
	if txt == "" {
		return ""
	}
	parties := strings.Split(strings.TrimSuffix(txt, "\n"), "\n")
	if len(parties) > n {
		parties = parties[len(parties)-n:]
	}
	return strings.Join(parties, "\n") + "\n"
}

// filtrer : les lignes qui correspondent au motif, les n dernières.
func filtrer(txt, motif string, n int) string {
	re := regexp.MustCompile(motif)
	var garde []string
	for _, l := range strings.Split(txt, "\n") {
		if re.MatchString(l) {
			garde = append(garde, l)
		}
	}
	if len(garde) > n {
		garde = garde[len(garde)-n:]
	}
	if len(garde) == 0 {
		return ""
	}
	return strings.Join(garde, "\n") + "\n"
}

// firstLine : la première ligne d'un texte.
func firstLine(txt string) string {
	l, _, _ := strings.Cut(txt, "\n")
	return l
}
