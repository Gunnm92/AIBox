package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const usageRunExe = `Usage : aibox run-exe PROGRAMME.exe [arguments…]   (ou double-clic dans Thunar)
  Proton : PROTON dans ~/.config/aibox/proton.conf (nom d'un dossier de
  ~/.local/share/Steam/compatibilitytools.d, ou chemin complet) ; sinon le
  proton-cachyos le plus récent ; sinon UMU-Proton, téléchargé une fois par umu.
  Préfixe Wine partagé : ~/.local/share/aibox/wine/default (WINEPREFIX=… pour un autre).
  Journal : ~/.cache/aibox/run-exe/<programme>.log`

// remplaceurUmu remplace le processus par umu-run, dans le dossier du programme,
// avec sa sortie ajoutée au journal (comme « exec umu-run >> journal 2>&1 »).
type remplaceurUmu func(argv, vars []string, dossier, journal string) error

// cmdRunExe : « aibox run-exe », lance un .exe ou un .msi avec Proton (umu-run).
func cmdRunExe(args []string, _ io.Writer, erreurs io.Writer) int {
	return runExe(envReel(), args, erreurs, lancerUmu)
}

// runExe prépare Proton, le préfixe et le journal, puis remplace le processus.
func runExe(e env, args []string, erreurs io.Writer, remplacer remplaceurUmu) int {
	if len(args) == 0 {
		fmt.Fprintln(erreurs, usageRunExe)
		return 1
	}
	file, err := filepath.Abs(args[0])
	if err == nil {
		file, err = filepath.EvalSymlinks(file)
	}
	if err != nil {
		fmt.Fprintf(erreurs, "%s introuvable.\n", args[0])
		return 1
	}
	if info, err := os.Stat(file); err != nil || info.IsDir() {
		fmt.Fprintf(erreurs, "%s introuvable.\n", file)
		return 1
	}

	dirs := []string{
		filepath.Join(e.home, ".local", "share", "Steam", "compatibilitytools.d"),
		filepath.Join(e.racine, "usr", "share", "steam", "compatibilitytools.d"),
	}
	proton := lireProtonConf(filepath.Join(e.home, ".config", "aibox", "proton.conf"))
	if proton != "" && !filepath.IsAbs(proton) {
		for _, d := range dirs {
			if info, err := os.Stat(filepath.Join(d, proton)); err == nil && info.IsDir() {
				proton = filepath.Join(d, proton)
				break
			}
		}
	}
	if proton == "" {
		proton = protonLePlusRecent(dirs)
	}

	vars := []string{}
	if proton != "" {
		vars = append(vars, "PROTONPATH="+proton)
	}
	wineprefix := os.Getenv("WINEPREFIX")
	if wineprefix == "" {
		wineprefix = filepath.Join(e.home, ".local", "share", "aibox", "wine", "default")
	}
	gameid := os.Getenv("GAMEID")
	if gameid == "" {
		gameid = "umu-default"
	}
	vars = append(vars, "WINEPREFIX="+wineprefix, "GAMEID="+gameid)

	dossierJournal := filepath.Join(e.home, ".cache", "aibox", "run-exe")
	if err := os.MkdirAll(wineprefix, 0o755); err != nil {
		fmt.Fprintf(erreurs, "aibox run-exe : %v\n", err)
		return 1
	}
	if err := os.MkdirAll(dossierJournal, 0o755); err != nil {
		fmt.Fprintf(erreurs, "aibox run-exe : %v\n", err)
		return 1
	}
	journal := filepath.Join(dossierJournal, filepath.Base(file)+".log")

	argv := []string{"umu-run"}
	if strings.HasSuffix(strings.ToLower(file), ".msi") {
		argv = append(argv, "msiexec", "/i", file)
	} else {
		argv = append(argv, file)
	}
	argv = append(argv, args[1:]...)

	entete := fmt.Sprintf("== %s : %s\nProton : %s  Préfixe : %s\n",
		time.Now().Format("Mon Jan _2 15:04:05 MST 2006"), strings.Join(argv[1:], " "),
		descriptionProton(proton), wineprefix)
	if err := ajouterJournal(journal, entete); err != nil {
		fmt.Fprintf(erreurs, "aibox run-exe : %v\n", err)
		return 1
	}

	if err := remplacer(argv, vars, filepath.Dir(file), journal); err != nil {
		fmt.Fprintf(erreurs, "aibox run-exe : %v\n", err)
		return 127
	}
	return 0
}

// lancerUmu : remplacement réel du processus.
func lancerUmu(argv, vars []string, dossier, journal string) error {
	chemin, err := exec.LookPath("umu-run")
	if err != nil {
		return err
	}
	if err := os.Chdir(dossier); err != nil {
		return err
	}
	f, err := os.OpenFile(journal, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if err := syscall.Dup2(int(f.Fd()), 1); err != nil {
		return err
	}
	if err := syscall.Dup2(int(f.Fd()), 2); err != nil {
		return err
	}
	return syscall.Exec(chemin, argv, append(os.Environ(), vars...))
}

// ajouterJournal : ajoute une entrée au journal du programme.
func ajouterJournal(journal, texte string) error {
	f, err := os.OpenFile(journal, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(texte)
	return err
}

// descriptionProton : le Proton choisi, ou UMU-Proton par défaut.
func descriptionProton(proton string) string {
	if proton == "" {
		return "UMU-Proton (par défaut)"
	}
	return proton
}

// motifProtonConf : une affectation PROTON=valeur (guillemets simples ou doubles acceptés).
var motifProtonConf = regexp.MustCompile(`^\s*PROTON=["']?([^"'\s]*)["']?\s*$`)

// lireProtonConf : la valeur de PROTON dans proton.conf (vide si absente).
// proton.conf est lu comme une simple affectation : pas de substitution de shell.
func lireProtonConf(chemin string) string {
	b, err := os.ReadFile(chemin)
	if err != nil {
		return ""
	}
	valeur := ""
	for _, l := range strings.Split(string(b), "\n") {
		if m := motifProtonConf.FindStringSubmatch(l); m != nil {
			valeur = m[1]
		}
	}
	return valeur
}

// protonLePlusRecent : le proton-cachyos au plus grand numéro de version, parmi les dossiers.
func protonLePlusRecent(dossiers []string) string {
	var candidats []string
	for _, d := range dossiers {
		motifs, _ := filepath.Glob(filepath.Join(d, "proton-cachyos*"))
		candidats = append(candidats, motifs...)
	}
	if len(candidats) == 0 {
		return ""
	}
	meilleur := candidats[0]
	for _, c := range candidats[1:] {
		if versionAvant(meilleur, c) {
			meilleur = c
		}
	}
	return meilleur
}

// versionAvant : vrai si a précède b au sens de sort -V (les nombres comptent par leur valeur).
func versionAvant(a, b string) bool {
	morceauxA := decoupe(a)
	morceauxB := decoupe(b)
	for i := 0; i < len(morceauxA) && i < len(morceauxB); i++ {
		x, y := morceauxA[i], morceauxB[i]
		nx, errX := strconv.Atoi(x)
		ny, errY := strconv.Atoi(y)
		if errX == nil && errY == nil {
			if nx != ny {
				return nx < ny
			}
			continue
		}
		if x != y {
			return x < y
		}
	}
	return len(morceauxA) < len(morceauxB)
}

// decoupe : une chaîne en morceaux de chiffres et de non-chiffres.
func decoupe(s string) []string {
	var res []string
	debut := 0
	for i := 1; i <= len(s); i++ {
		if i == len(s) || estChiffre(s[i]) != estChiffre(s[i-1]) {
			res = append(res, s[debut:i])
			debut = i
		}
	}
	return res
}

func estChiffre(c byte) bool { return c >= '0' && c <= '9' }
