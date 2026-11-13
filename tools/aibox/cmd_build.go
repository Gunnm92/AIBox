package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const remoteAibox = "https://github.com/Gunnm92/AIBox.git"

const usageBuild = `Usage : sudo aibox build [--ref BRANCHE|COMMIT] [--fetch-only] | --import DEPOT
  aibox build                met à jour la copie et construit origin/main (ou --ref)
  aibox build --fetch-only   met à jour la copie seulement (affiche le commit)
  aibox build --import DEPOT reprend keys/ et les caches d'un dépôt (une fois)
Tout est dans /var/lib/aibox/state/build (disque de données). Seul ce qui est poussé
sur GitHub est construit.`

// optionsBuild : les options de aibox build.
type optionsBuild struct {
	ref       string
	fetchOnly bool
	depotImport string
}

// cmdBuild : « aibox build », construit l'ISO depuis GitHub (origin/<ref>).
func cmdBuild(args []string, sortie, erreurs io.Writer) int {
	o := optionsBuild{ref: "main"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ref":
			if i+1 >= len(args) {
				fmt.Fprintln(erreurs, usageBuild)
				return 1
			}
			o.ref = args[i+1]
			i++
		case "--fetch-only":
			o.fetchOnly = true
		case "--import":
			if i+1 >= len(args) {
				fmt.Fprintln(erreurs, usageBuild)
				return 1
			}
			o.depotImport = args[i+1]
			i++
		default:
			fmt.Fprintln(erreurs, usageBuild)
			return 1
		}
	}
	if os.Geteuid() != 0 {
		fmt.Fprintf(erreurs, "À lancer en root : sudo aibox build %s\n", strings.Join(args, " "))
		return 1
	}
	return build(envReel(), o, sortie, erreurs)
}

// build : reprise des secrets, copie du dépôt, garde-fous, build et journal.
func build(e env, o optionsBuild, sortie, erreurs io.Writer) int {
	if !montageExiste(e, "var/lib/aibox/state") {
		fmt.Fprintln(erreurs, "Pas de disque de données (mode éphémère) : construire l'ISO demande le disque.")
		return 1
	}
	base := e.chemin("var/lib/aibox/state/build")
	for _, d := range []string{"work", "out"} {
		os.MkdirAll(filepath.Join(base, d), 0o755)
	}
	keys := filepath.Join(base, "keys")
	os.MkdirAll(keys, 0o700)
	os.Chmod(keys, 0o700)

	// Une seule exécution à la fois : verrou flock, gardé ouvert jusqu'à la fin.
	verrou, err := os.OpenFile(filepath.Join(base, "verrou"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		fmt.Fprintf(erreurs, "aibox build : %v\n", err)
		return 1
	}
	defer verrou.Close()
	if err := syscall.Flock(int(verrou.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		fmt.Fprintln(erreurs, "Un build est déjà en cours.")
		return 1
	}

	if o.depotImport != "" {
		return importerDepot(e, o.depotImport, base, sortie, erreurs)
	}

	// Copie du dépôt, remise à origin/<ref>.
	src := filepath.Join(base, "src")
	remote := os.Getenv("AIBOX_BUILD_REMOTE")
	if remote == "" {
		remote = remoteAibox
	}
	if _, err := os.Stat(filepath.Join(src, ".git")); err != nil {
		if _, err := e.lancer(nil, "git", "clone", "-q", remote, src); err != nil {
			fmt.Fprintf(erreurs, "aibox build : clonage impossible : %v\n", err)
			return 1
		}
	}
	git := func(args ...string) (string, error) {
		return e.lancer(nil, "git", append([]string{"-C", src}, args...)...)
	}
	git("remote", "set-url", "origin", remote)
	if _, err := git("fetch", "-q", "origin"); err != nil {
		fmt.Fprintf(erreurs, "aibox build : fetch impossible : %v\n", err)
		return 1
	}
	cible := "origin/" + o.ref
	if _, err := git("rev-parse", "-q", "--verify", cible+"^{commit}"); err != nil {
		cible = o.ref
	}
	if _, err := git("reset", "-q", "--hard", cible); err != nil {
		fmt.Fprintf(erreurs, "aibox build : reset impossible sur %s : %v\n", cible, err)
		return 1
	}
	git("clean", "-q", "-fdx")
	sortieCommit, _ := git("rev-parse", "--short=7", "HEAD")
	commit := strings.TrimSpace(sortieCommit)
	if o.fetchOnly {
		fmt.Fprintln(sortie, commit)
		return 0
	}

	// Garde-fous : secrets présents, place libre.
	if info, err := os.Stat(filepath.Join(keys, "authorized_keys")); err != nil || info.Size() == 0 {
		fmt.Fprintf(erreurs, "Pas de %s/authorized_keys : reprendre les secrets avec --import.\n", keys)
		return 1
	}
	libreGo := espaceLibre(e.chemin("var/lib/aibox/state"))
	if libreGo < 12 {
		fmt.Fprintf(erreurs, "Place insuffisante sur le disque de données : %d Go (12 au moins).\n", libreGo)
		return 1
	}

	// Build : build.sh du dépôt, sortie copiée dans build.log.
	debut := time.Now()
	fmt.Fprintf(sortie, "Build de %s (origin/%s)…\n", commit, o.ref)
	journal, err := os.Create(filepath.Join(base, "build.log"))
	if err != nil {
		fmt.Fprintf(erreurs, "aibox build : %v\n", err)
		return 1
	}
	defer journal.Close()
	vars := []string{"AIBOX_KEYS=" + keys, "AIBOX_WORK=" + filepath.Join(base, "work"), "AIBOX_OUT=" + filepath.Join(base, "out")}
	statut := "ok"
	if code := e.flux(vars, io.MultiWriter(sortie, journal), "nice", "-n", "10", filepath.Join(src, "build.sh")); code != 0 {
		statut = "echec"
	}
	// La version est la dernière ligne « Version : » du journal.
	journal.Close()
	journalTxt, _ := os.ReadFile(filepath.Join(base, "build.log"))
	version := derniereVersion(string(journalTxt))
	iso := filepath.Join(base, "out", "aibox-"+version+"-x86_64.iso")
	if statut != "ok" {
		statut = "echec"
	} else if _, err := os.Stat(iso); err != nil {
		statut = "echec"
	}

	enregistrerResultat(filepath.Join(base, "dernier-build.json"), resultatBuild{
		Statut: statut, Version: version, Commit: commit, Ref: o.ref, ISO: iso,
		Date: time.Now().UTC().Format("2006-01-02T15:04:05Z"), DureeS: int(time.Since(debut).Seconds()),
	})
	if statut != "ok" {
		fmt.Fprintf(erreurs, "Échec du build : voir %s\n", filepath.Join(base, "build.log"))
		return 1
	}
	fmt.Fprintf(sortie, "ISO : %s\n", iso)
	return 0
}

// resultatBuild : contenu de dernier-build.json (mêmes clés que le script).
type resultatBuild struct {
	Statut  string `json:"statut"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Ref     string `json:"ref"`
	ISO     string `json:"iso"`
	Date    string `json:"date"`
	DureeS  int    `json:"duree_s"`
}

func enregistrerResultat(chemin string, r resultatBuild) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(chemin, append(b, '\n'), 0o644)
}

// derniereVersion : la valeur de la dernière ligne « Version : » du journal.
func derniereVersion(journal string) string {
	version := ""
	for _, l := range strings.Split(journal, "\n") {
		if v, ok := strings.CutPrefix(l, "Version : "); ok {
			version = strings.TrimSpace(v)
		}
	}
	return version
}

// espaceLibre : Go libres sur le système de fichiers, arrondis au supérieur (df -BG).
// Variable pour que les tests fournissent une valeur fixe.
var espaceLibre = func(chemin string) uint64 {
	var s syscall.Statfs_t
	if err := syscall.Statfs(chemin, &s); err != nil {
		return 0
	}
	octets := s.Bavail * uint64(s.Bsize)
	const gio = 1 << 30
	return (octets + gio - 1) / gio
}

// importerDepot reprend les secrets (keys/, sauf README.md) et les caches
// (work/pkgcache, aur, dl) d'un dépôt, une fois.
func importerDepot(e env, depot, base string, sortie, erreurs io.Writer) int {
	if info, err := os.Stat(filepath.Join(depot, "keys")); err != nil || !info.IsDir() {
		fmt.Fprintf(erreurs, "%s/keys introuvable.\n", depot)
		return 1
	}
	entrees, _ := os.ReadDir(filepath.Join(depot, "keys"))
	for _, f := range entrees {
		if f.IsDir() || f.Name() == "README.md" {
			continue
		}
		contenu, err := os.ReadFile(filepath.Join(depot, "keys", f.Name()))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(base, "keys", f.Name()), contenu, 0o600); err != nil {
			fmt.Fprintf(erreurs, "aibox build : %v\n", err)
			return 1
		}
		os.Chmod(filepath.Join(base, "keys", f.Name()), 0o600)
		fmt.Fprintf(sortie, "Secret repris : %s\n", f.Name())
	}
	for _, d := range []string{"pkgcache", "aur", "dl"} {
		src := filepath.Join(depot, "work", d)
		dst := filepath.Join(base, "work", d)
		if info, err := os.Stat(src); err != nil || !info.IsDir() {
			continue
		}
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if _, err := e.lancer(nil, "cp", "-a", "--reflink=auto", src, dst); err != nil {
			fmt.Fprintf(erreurs, "aibox build : copie de %s impossible : %v\n", d, err)
			return 1
		}
		taille := "?"
		if sortieDu, err := e.lancer(nil, "du", "-sh", dst); err == nil && len(strings.Fields(sortieDu)) > 0 {
			taille = strings.Fields(sortieDu)[0]
		}
		fmt.Fprintf(sortie, "Cache repris : work/%s (%s)\n", d, taille)
	}
	return 0
}
