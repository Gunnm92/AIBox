package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// budgetSystemeMio : Shmem toléré par le système (les tmpfs ne se rendent pas au noyau).
const budgetSystemeMio = 6 * 1024

// cmdMem : « aibox mem », état de la mémoire du système live et de la clé USB.
func cmdMem(args []string, sortie, erreurs io.Writer) int {
	if len(args) > 0 {
		fmt.Fprintln(erreurs, "Usage : aibox mem")
		return 2
	}
	afficherMem(envReel(), sortie)
	return 0
}

// afficherMem écrit l'état de la mémoire. Sépare la lecture du système (env)
// de la mise en forme, pour que les tests fournissent un faux système.
func afficherMem(e env, w io.Writer) {
	total := e.meminfo("MemTotal")
	dispo := e.meminfo("MemAvailable")
	shmem := e.meminfo("Shmem")
	cache := e.meminfo("Cached")
	image := mio(e.tailleDossier("run/archiso/copytoram"))

	fmt.Fprintln(w, "== Mémoire (Mio)")
	ecrireLigne(w, "Total", fmt.Sprint(total))
	ecrireLigne(w, "Disponible", fmt.Sprint(dispo))
	ecrireLigne(w, "Cache récupérable (Cached)", fmt.Sprint(cache-shmem))
	ecrireLigne(w, "Bloquée par le système (Shmem)", fmt.Sprint(shmem))
	ecrireLigne(w, "  dont image en RAM", fmt.Sprint(image))
	if shmem <= budgetSystemeMio {
		fmt.Fprintf(w, "  Budget système (≤ %d Mio) : OK\n", budgetSystemeMio)
	} else {
		fmt.Fprintf(w, "  Budget système (≤ %d Mio) : DÉPASSÉ\n", budgetSystemeMio)
	}

	fmt.Fprintln(w, "\n== GPU (mémoire unifiée)")
	if limite, ok := e.lireInt("sys/module/ttm/parameters/pages_limit"); ok {
		pool, _ := e.lireInt("sys/module/ttm/parameters/page_pool_size")
		ecrireLigne(w, "Plafond GTT (ttm.pages_limit)", fmt.Sprint(limite/256))
		ecrireLigne(w, "Cache de pages (page_pool_size)", fmt.Sprint(pool/256))
	}
	for _, dev := range e.cartesDrm() {
		vram := octetsEnMio(dev, "mem_info_vram_total")
		gttUtilise := octetsEnMio(dev, "mem_info_gtt_used")
		gttTotal := octetsEnMio(dev, "mem_info_gtt_total")
		ecrireLigne(w, "VRAM réservée par le BIOS (UMA)", fmt.Sprint(vram))
		ecrireLigne(w, "GTT utilisée / total", fmt.Sprintf("%d / %d", gttUtilise, gttTotal))
		if vram > 1024 {
			fmt.Fprintln(w, "  Attention : UMA > 1 Gio, la mettre au minimum (512 Mo) dans le BIOS.")
		}
	}

	fmt.Fprintln(w, "\n== Disque de données")
	if e.existe("run/aibox/data-present") {
		source := "?"
		if m, ok := e.trouverMontage("/home"); ok {
			source = strings.SplitN(m.source, "[", 2)[0]
		}
		fmt.Fprintf(w, "  AIBOXDATA monté (%s)\n", source)
		if taille, utilise, dispo, ok := e.statfs("home"); ok {
			fmt.Fprintf(w, "  Taille %s, utilisé %s, disponible %s\n", humain(taille), humain(utilise), humain(dispo))
		}
	} else {
		fmt.Fprintln(w, "  Aucun : MODE ÉPHÉMÈRE (rien n'est conservé). sudo aibox setup-disk /dev/nvmeXnY")
	}

	fmt.Fprintln(w, "\n== Image système")
	if e.existe("run/archiso/copytoram") {
		for _, f := range e.fichiers("run/archiso/copytoram") {
			fmt.Fprintf(w, "  %s  %dM\n", f.nom, mio(uint64(f.taille)))
		}
	} else {
		fmt.Fprintln(w, "  Pas de copie en RAM (copytoram=n) : la clé doit rester branchée.")
	}

	fmt.Fprintln(w, "\n== Clé USB")
	if _, ok := e.trouverMontage("/run/archiso/bootmnt"); ok {
		fmt.Fprintln(w, "  Encore montée sur /run/archiso/bootmnt : ne pas la retirer.")
	} else {
		fmt.Fprintln(w, "  Libérée : la clé peut être retirée.")
	}

	fmt.Fprintln(w, "\n== tmpfs (taille max / utilisé)")
	fmt.Fprintf(w, "  %-30s %10s %10s\n", "Cible", "Taille", "Utilisé")
	for _, m := range e.montages() {
		if m.typ != "tmpfs" {
			continue
		}
		if taille, utilise, _, ok := e.statfs(m.point); ok {
			fmt.Fprintf(w, "  %-30s %10s %10s\n", m.point, humain(taille), humain(utilise))
		}
	}

	fmt.Fprintln(w, "\n== Overlay (écritures dans le système)")
	if taille, utilise, dispo, ok := e.statfs("run/archiso/cowspace"); ok {
		fmt.Fprintf(w, "  Taille %s, utilisé %s, disponible %s\n", humain(taille), humain(utilise), humain(dispo))
	}

	fmt.Fprintln(w, "\n== zram")
	if sortie, err := e.lancer("zramctl", "--noheadings"); err == nil && strings.TrimSpace(sortie) != "" {
		complet, _ := e.lancer("zramctl")
		for _, l := range strings.Split(strings.TrimRight(complet, "\n"), "\n") {
			fmt.Fprintf(w, "  %s\n", l)
		}
	} else {
		fmt.Fprintln(w, "  aucun")
	}

	fmt.Fprintln(w, "\n== Démarrage")
	if sortie, err := e.lancer("systemd-analyze"); err == nil {
		premiere, _, _ := strings.Cut(sortie, "\n")
		fmt.Fprintf(w, "  %s\n", premiere)
	}
}

// meminfo : valeur en Mio d'une ligne de /proc/meminfo (exprimée en kB).
func (e env) meminfo(cle string) int64 {
	txt, _ := e.lire("proc/meminfo")
	for _, l := range strings.Split(txt, "\n") {
		f := strings.Fields(l)
		if len(f) >= 2 && f[0] == cle+":" {
			kb, _ := parseEntier(f[1])
			return kb / 1024
		}
	}
	return 0
}

// cartesDrm : les dossiers device des cartes graphiques qui exposent la mémoire GTT.
func (e env) cartesDrm() []string {
	var res []string
	motifs, _ := filepath.Glob(e.chemin("sys/class/drm/card*/device"))
	for _, dev := range motifs {
		if _, err := os.Stat(filepath.Join(dev, "mem_info_gtt_total")); err == nil {
			res = append(res, dev)
		}
	}
	return res
}

// octetsEnMio : valeur d'un fichier sysfs (octets), en Mio.
func octetsEnMio(dossier, fichier string) int64 {
	b, err := os.ReadFile(filepath.Join(dossier, fichier))
	if err != nil {
		return 0
	}
	n, _ := parseEntier(strings.TrimSpace(string(b)))
	return n / 1048576
}

// fichierDossier : nom et taille d'un fichier d'un dossier.
type fichierDossier struct {
	nom    string
	taille int64
}

// fichiers : les fichiers (pas les sous-dossiers) d'un dossier système.
func (e env) fichiers(rel string) []fichierDossier {
	entrees, err := os.ReadDir(e.chemin(rel))
	if err != nil {
		return nil
	}
	var res []fichierDossier
	for _, d := range entrees {
		if d.IsDir() {
			continue
		}
		if info, err := d.Info(); err == nil {
			res = append(res, fichierDossier{nom: d.Name(), taille: info.Size()})
		}
	}
	return res
}
