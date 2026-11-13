package main

import (
	"strings"
	"testing"
)

func TestDepotsEtMajPacman(t *testing.T) {
	sortie := "core mesa 26.1.0-1\nextra mesa 26.1.1-1\ncore openssh 10.0p1-1\nmultilib lib32-mesa 26.1.0-1\n"
	dispo := depotsParPaquet(sortie)
	if dispo["mesa"][0] != "26.1.0-1" || dispo["mesa"][1] != "core" {
		t.Errorf("première occurrence attendue, obtenu %v", dispo["mesa"])
	}
	liste := "mesa 26.1.0-1\nopenssh 10.0p1-1\nfiglet 2.2.5-1\n"
	vercmp := func(a, b string) int {
		if a == b {
			return 0
		}
		if a > b {
			return 1
		}
		return -1
	}
	maj := paquetsAMettreAJour(liste, dispo, vercmp)
	if len(maj) != 0 {
		// mesa : 26.1.0 vs 26.1.0 (dépôt core) → pas de mise à jour à ce niveau
		t.Errorf("maj inattendue : %+v", maj)
	}
	maj = paquetsAMettreAJour("mesa 26.0.0-1\n", dispo, vercmp)
	if len(maj) != 1 || maj[0].nouvelle != "26.1.0-1" || maj[0].depot != "core" {
		t.Errorf("maj = %+v", maj)
	}
	if got := texteMaj(maj); got != "mesa 26.0.0-1 26.1.0-1 core\n" {
		t.Errorf("texte = %q", got)
	}
}

func TestExtraireDecision(t *testing.T) {
	cas := []struct {
		contenu string
		ok      bool
		maj     bool
	}{
		{`{"mettre_a_jour": true, "raison": "noyau", "paquets_importants": []}`, true, true},
		{"<think>je réfléchis {x}</think>\nRéponse : {\"mettre_a_jour\": false, \"raison\": \"mineurs\"}", true, false},
		{`{"autre": 1}`, false, false},
		{"pas de json", false, false},
	}
	for _, c := range cas {
		d, ok := extraireDecision(c.contenu)
		if ok != c.ok {
			t.Errorf("%q : ok = %v", c.contenu, ok)
			continue
		}
		if ok && d["mettre_a_jour"].(bool) != c.maj {
			t.Errorf("%q : mettre_a_jour = %v", c.contenu, d["mettre_a_jour"])
		}
	}
}

func TestPromptMajReprendLesCas(t *testing.T) {
	p := promptMaj("2026.11.13.0405-7fa8dfc", 3, 2, "a 1 2\nb 3 4")
	for _, attendu := range []string{"lemonade-server", "mesa", "21 jours", `"mettre_a_jour": true|false`, "a 1 2"} {
		if !strings.Contains(p, attendu) {
			t.Errorf("prompt sans %q", attendu)
		}
	}
}

func TestGardeJournal(t *testing.T) {
	long := make([]string, 2500)
	if got := len(gardeJournal(long)); got != 1500 {
		t.Errorf("journal gardé = %d lignes, attendu 1500", got)
	}
	if got := len(gardeJournal(long[:100])); got != 100 {
		t.Errorf("journal court modifié : %d", got)
	}
}

func TestProfilDroits(t *testing.T) {
	pd := `file_permissions=(
  ["/usr/local/bin/aibox"]="0:0:755"
  ["/etc/sudoers.d"]="0:0:750"
  ["/usr/local/lib/aibox/"]="0:0:755"
)`
	d := profilDroits(pd)
	if d["/usr/local/bin/aibox"] != "755" || d["/etc/sudoers.d"] != "750" {
		t.Errorf("droits = %v", d)
	}
	if _, ok := d["/usr/local/lib/aibox/"]; ok {
		t.Error("les dossiers ne doivent pas être listés")
	}
}
