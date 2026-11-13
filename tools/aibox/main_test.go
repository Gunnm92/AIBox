package main

import (
	"bytes"
	"strings"
	"testing"
)

// Chaque commande a un nom unique, un script et une rubrique connue.
func TestCommandesCoherentes(t *testing.T) {
	vues := map[string]bool{}
	rubriques := map[string]bool{}
	for _, g := range groupes {
		rubriques[g] = true
	}
	for _, c := range commandes {
		if vues[c.nom] {
			t.Errorf("nom en double : %s", c.nom)
		}
		vues[c.nom] = true
		if c.exe == "" {
			t.Errorf("%s : script vide", c.nom)
		}
		if !rubriques[c.groupe] {
			t.Errorf("%s : rubrique inconnue %q", c.nom, c.groupe)
		}
		if len(c.usages) == 0 {
			t.Errorf("%s : pas d'usage", c.nom)
		}
	}
}

// L'aide cite chaque commande, et ne se trompe pas de rubrique.
func TestAideComplete(t *testing.T) {
	var b bytes.Buffer
	ecrireAide(&b)
	aide := b.String()
	for _, c := range commandes {
		if !strings.Contains(aide, "  "+c.nom+" ") {
			t.Errorf("l'aide ne cite pas %s", c.nom)
		}
		if !strings.Contains(aide, "script : "+c.exe) {
			t.Errorf("l'aide ne cite pas le script %s de %s", c.exe, c.nom)
		}
	}
	for _, g := range groupes {
		if !strings.Contains(aide, "\n"+g+"\n") {
			t.Errorf("rubrique absente de l'aide : %s", g)
		}
	}
}

func TestAideSansArgument(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"-h"}, {"help"}} {
		var sortie, erreurs bytes.Buffer
		if code := run(args, &sortie, &erreurs); code != 0 {
			t.Errorf("run(%v) = %d, attendu 0", args, code)
		}
		if !strings.Contains(sortie.String(), "Mise à jour") {
			t.Errorf("run(%v) n'affiche pas l'aide", args)
		}
	}
}

func TestCommandeInconnue(t *testing.T) {
	var sortie, erreurs bytes.Buffer
	if code := run([]string{"n-existe-pas"}, &sortie, &erreurs); code != 2 {
		t.Errorf("code = %d, attendu 2", code)
	}
	if !strings.Contains(erreurs.String(), "commande inconnue") {
		t.Errorf("message d'erreur absent : %q", erreurs.String())
	}
}
