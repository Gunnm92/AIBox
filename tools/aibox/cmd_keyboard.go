package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const usageKeyboard = `Usage : aibox keyboard [mac | pc | <disposition> [variante]]
  aibox keyboard              affiche la disposition actuelle
  aibox keyboard mac          Mac français, depuis un navigateur sous macOS (par défaut)
  aibox keyboard pc           PC français (AZERTY)
  aibox keyboard <disposition> [variante]   toute disposition xkb (ex. us, ch fr)
C'est wayvnc qui traduit les touches reçues de noVNC. Le réglage est gardé dans
~/.config/aibox/keyboard.env ; wayvnc est relancé, noVNC se reconnecte tout seul.`

// cmdKeyboard : « aibox keyboard », disposition du clavier du bureau d'administration.
func cmdKeyboard(args []string, sortie, erreurs io.Writer) int {
	return keyboard(envReel(), args, sortie, erreurs)
}

// keyboard affiche ou change la disposition, après l'avoir vérifiée dans les
// fichiers xkb du système.
func keyboard(e env, args []string, sortie, erreurs io.Writer) int {
	if len(args) == 0 {
		valeurs := lireEnv(filepath.Join(e.racine, "etc", "aibox", "keyboard.env"))
		for k, v := range lireEnv(filepath.Join(e.home, ".config", "aibox", "keyboard.env")) {
			valeurs[k] = v
		}
		fmt.Fprintf(sortie, "Clavier : %s", valeurs["KEYBOARD_LAYOUT"])
		if v := valeurs["KEYBOARD_VARIANT"]; v != "" {
			fmt.Fprintf(sortie, " (%s)", v)
		}
		fmt.Fprintln(sortie)
		return 0
	}

	var layout, variante string
	switch args[0] {
	case "mac":
		layout, variante = "aibox", "fr_mac"
	case "pc":
		layout, variante = "fr", ""
	default:
		layout = args[0]
		if len(args) > 1 {
			variante = args[1]
		}
	}
	if code := verifierDisposition(e, layout, variante, erreurs); code != 0 {
		return code
	}

	conf := filepath.Join(e.home, ".config", "aibox", "keyboard.env")
	if err := os.MkdirAll(filepath.Dir(conf), 0o755); err != nil {
		fmt.Fprintf(erreurs, "aibox keyboard : %v\n", err)
		return 1
	}
	contenu := fmt.Sprintf("KEYBOARD_LAYOUT=%s\nKEYBOARD_VARIANT=%s\n", layout, variante)
	if err := os.WriteFile(conf, []byte(contenu), 0o644); err != nil {
		fmt.Fprintf(erreurs, "aibox keyboard : %v\n", err)
		return 1
	}
	if code := e.executer(nil, "systemctl", "--user", "restart", "aibox-admin-wayvnc.service"); code != 0 {
		return code
	}
	if variante != "" {
		fmt.Fprintf(sortie, "Clavier : %s (%s). noVNC se reconnecte.\n", layout, variante)
	} else {
		fmt.Fprintf(sortie, "Clavier : %s. noVNC se reconnecte.\n", layout)
	}
	return 0
}

// verifierDisposition : la disposition existe dans /etc/xkb/symbols (disposition
// du système) ou dans la liste de base xkb, et la variante aussi. Code 0 si oui.
func verifierDisposition(e env, layout, variante string, erreurs io.Writer) int {
	symboles := filepath.Join(e.racine, "etc", "xkb", "symbols", layout)
	if b, err := os.ReadFile(symboles); err == nil {
		if variante != "" && !strings.Contains(string(b), `xkb_symbols "`+variante+`"`) {
			fmt.Fprintf(erreurs, "Variante inconnue pour %s : %s\n", layout, variante)
			return 1
		}
		return 0
	}

	b, err := os.ReadFile(filepath.Join(e.racine, "usr", "share", "X11", "xkb", "rules", "base.lst"))
	if err != nil {
		fmt.Fprintf(erreurs, "aibox keyboard : liste xkb illisible : %v\n", err)
		return 1
	}
	liste := string(b)
	if !regexp.MustCompile(`(?m)^ +` + regexp.QuoteMeta(layout) + ` +`).MatchString(sectionLayouts(liste)) {
		fmt.Fprintf(erreurs, "Disposition inconnue : %s\n", layout)
		return 1
	}
	if variante != "" && !regexp.MustCompile(`(?m)^ +`+regexp.QuoteMeta(variante)+` +`+regexp.QuoteMeta(layout)+`:`).MatchString(liste) {
		fmt.Fprintf(erreurs, "Variante inconnue pour %s : %s\n", layout, variante)
		fmt.Fprintln(erreurs, "Variantes disponibles :")
		for _, m := range regexp.MustCompile(`(?m)^ +([^ ]+) +`+regexp.QuoteMeta(layout)+`:`).FindAllStringSubmatch(liste, -1) {
			fmt.Fprintf(erreurs, "  %s\n", m[1])
		}
		return 1
	}
	return 0
}

// sectionLayouts : la section « ! layout » de base.lst, jusqu'à la ligne vide.
func sectionLayouts(liste string) string {
	_, apres, ok := strings.Cut(liste, "! layout")
	if !ok {
		return ""
	}
	if i := strings.Index(apres, "\n\n"); i >= 0 {
		return apres[:i]
	}
	return apres
}

// lireEnv : affectations KEY=VALUE d'un fichier (guillemets retirés).
func lireEnv(chemin string) map[string]string {
	res := map[string]string{}
	b, err := os.ReadFile(chemin)
	if err != nil {
		return res
	}
	for _, l := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(l), "=")
		if !ok || strings.HasPrefix(k, "#") {
			continue
		}
		res[k] = strings.Trim(v, `"'`)
	}
	return res
}
