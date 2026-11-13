package main

import (
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// env : accès au système, injectable dans les tests. racine remplace « / »
// pour les chemins lus (/proc, /sys, /run, /home) ; runtime et home sont
// XDG_RUNTIME_DIR et $HOME. lancer capture la sortie d'une commande externe ;
// executer la lance avec les entrées-sorties du processus et renvoie son code.
// Les variables d'environnement supplémentaires passent en premier argument.
type env struct {
	racine   string
	runtime  string
	home     string
	lancer   func(vars []string, nom string, args ...string) (string, error)
	executer func(vars []string, nom string, args ...string) int
}

// envReel : le vrai système.
func envReel() env {
	runtime := os.Getenv("XDG_RUNTIME_DIR")
	if runtime == "" {
		runtime = fmt.Sprintf("/run/user/%d", os.Getuid())
	}
	return env{
		racine:  "/",
		runtime: runtime,
		home:    os.Getenv("HOME"),
		lancer: func(vars []string, nom string, args ...string) (string, error) {
			cmd := exec.Command(nom, args...)
			cmd.Env = append(os.Environ(), vars...)
			out, err := cmd.Output()
			return string(out), err
		},
		executer: func(vars []string, nom string, args ...string) int {
			cmd := exec.Command(nom, args...)
			cmd.Env = append(os.Environ(), vars...)
			cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				if sortie, ok := err.(*exec.ExitError); ok {
					return sortie.ExitCode()
				}
				fmt.Fprintf(os.Stderr, "aibox : %s : %v\n", nom, err)
				return 127
			}
			return 0
		},
	}
}

// chemin : le chemin réel d'un chemin système (« proc/meminfo » → racine/proc/meminfo).
func (e env) chemin(rel string) string {
	return filepath.Join(e.racine, rel)
}

// lire : contenu d'un fichier, sans espaces de fin de ligne.
func (e env) lire(rel string) (string, bool) {
	b, err := os.ReadFile(e.chemin(rel))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

// lireInt : entier lu dans un fichier, 0 si absent ou illisible.
func (e env) lireInt(rel string) (int64, bool) {
	s, ok := e.lire(rel)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil
}

// existe : vrai si le chemin système existe (fichier ou dossier).
func (e env) existe(rel string) bool {
	_, err := os.Stat(e.chemin(rel))
	return err == nil
}

// montage : une ligne de /proc/self/mountinfo.
type montage struct {
	point  string
	typ    string
	source string
}

// montages : la table des montages, lue dans /proc/self/mountinfo.
func (e env) montages() []montage {
	txt, _ := e.lire("proc/self/mountinfo")
	var res []montage
	for _, l := range strings.Split(txt, "\n") {
		gauche, droite, ok := strings.Cut(l, " - ")
		if !ok {
			continue
		}
		g := strings.Fields(gauche)
		d := strings.Fields(droite)
		if len(g) < 5 || len(d) < 2 {
			continue
		}
		res = append(res, montage{
			point:  desechapper(g[4]),
			typ:    d[0],
			source: d[1],
		})
	}
	return res
}

// trouverMontage : le montage dont le point est exactement p.
func (e env) trouverMontage(p string) (montage, bool) {
	for _, m := range e.montages() {
		if m.point == p {
			return m, true
		}
	}
	return montage{}, false
}

// desechapper : /proc n'écrit pas les espaces telles quelles (\040, \011, \012, \134).
func desechapper(s string) string {
	r := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return r.Replace(s)
}

// statfs : taille, utilisé et disponible (octets) d'un système de fichiers.
func (e env) statfs(rel string) (taille, utilise, dispo uint64, ok bool) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(e.chemin(rel), &s); err != nil {
		return 0, 0, 0, false
	}
	// df compte en taille de fragment (f_frsize), pas en taille de bloc.
	bs := uint64(s.Frsize)
	if bs == 0 {
		bs = uint64(s.Bsize)
	}
	return s.Blocks * bs, (s.Blocks - s.Bfree) * bs, s.Bavail * bs, true
}

// tailleDossier : somme des tailles des fichiers d'un dossier (en octets).
func (e env) tailleDossier(rel string) uint64 {
	var total uint64
	_ = filepath.WalkDir(e.chemin(rel), func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += uint64(info.Size())
			}
		}
		return nil
	})
	return total
}

// humain : taille lisible comme df -h : arrondi au supérieur, virgule décimale.
func humain(n uint64) string {
	unites := []string{"", "K", "M", "G", "T"}
	f := float64(n)
	i := 0
	for f >= 1024 && i < len(unites)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d", n)
	}
	if f < 10 {
		return strings.Replace(fmt.Sprintf("%.1f", math.Ceil(f*10)/10), ".", ",", 1) + unites[i]
	}
	return fmt.Sprintf("%.0f%s", math.Ceil(f), unites[i])
}

// mio : mébioctets, arrondis au supérieur comme du -m.
func mio(octets uint64) int64 {
	return int64((octets + 1048575) / 1048576)
}

// ecrireLigne : libellé et valeur alignés, comme printf '  %-34s %8s'.
func ecrireLigne(w io.Writer, libelle string, valeur string) {
	fmt.Fprintf(w, "  %-34s %8s\n", libelle, valeur)
}

// parseEntier : entier décimal (valeurs lues dans /proc et /sys).
func parseEntier(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
