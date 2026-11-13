package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// paquetMaj : un paquet de l'image dont une version plus récente existe dans les dépôts.
type paquetMaj struct {
	nom, ancienne, nouvelle, depot string
}

// depotsParPaquet lit la sortie de « pacman -Sl » (dépôt nom version [installé]) :
// pour chaque nom, la première occurrence (ordre de pacman.conf), comme le script.
func depotsParPaquet(sortie string) map[string][2]string {
	res := map[string][2]string{}
	for _, l := range strings.Split(sortie, "\n") {
		champs := strings.Fields(l)
		if len(champs) < 3 {
			continue
		}
		depot, nom, version := champs[0], champs[1], champs[2]
		if _, vu := res[nom]; !vu {
			res[nom] = [2]string{version, depot}
		}
	}
	return res
}

// paquetsAMettreAJour compare la liste de l'image (« nom version ») aux dépôts.
// vercmp renvoie > 0 si la première version est plus récente (comme pacman).
func paquetsAMettreAJour(liste string, dispo map[string][2]string, vercmp func(a, b string) int) []paquetMaj {
	var res []paquetMaj
	for _, l := range strings.Split(liste, "\n") {
		champs := strings.Fields(l)
		if len(champs) < 2 {
			continue
		}
		nom, ancienne := champs[0], champs[1]
		d, ok := dispo[nom]
		if !ok || d[0] == ancienne {
			continue
		}
		if vercmp(d[0], ancienne) > 0 {
			res = append(res, paquetMaj{nom: nom, ancienne: ancienne, nouvelle: d[0], depot: d[1]})
		}
	}
	return res
}

// texteMaj : une ligne « nom ancienne nouvelle dépôt » par paquet (derniers-paquets.txt).
func texteMaj(paquets []paquetMaj) string {
	var b strings.Builder
	for _, p := range paquets {
		fmt.Fprintf(&b, "%s %s %s %s\n", p.nom, p.ancienne, p.nouvelle, p.depot)
	}
	return b.String()
}

var motifThink = regexp.MustCompile(`(?s)<think>.*?</think>`)

// extraireDecision renvoie le dernier objet JSON qui contient « mettre_a_jour »
// (booléen), après avoir retiré les blocs <think>. Le modèle peut réfléchir avant.
func extraireDecision(contenu string) (map[string]any, bool) {
	texte := motifThink.ReplaceAllString(contenu, "")
	var trouve map[string]any
	for i := 0; i < len(texte); i++ {
		if texte[i] != '{' {
			continue
		}
		var d map[string]any
		dec := json.NewDecoder(strings.NewReader(texte[i:]))
		if err := dec.Decode(&d); err != nil {
			continue
		}
		if _, ok := d["mettre_a_jour"].(bool); ok {
			trouve = d
		}
	}
	return trouve, trouve != nil
}

// promptMaj : la consigne donnée au LLM local (texte identique à celui du script).
func promptMaj(versionImage string, age int, count int, paquets string) string {
	return fmt.Sprintf(`Tu décides si l'image système d'aibox (CachyOS en RAM : jeux Steam sur TV, LLM locaux avec Lemonade sur un AMD Ryzen AI Max+ 395 / Radeon 8060S) doit être reconstruite cette nuit. Reconstruire coûte quelques minutes et demande un redémarrage plus tard, décidé par l'utilisateur.

Image installée : %s (âge : %d jours). %d paquets ont une version plus récente :
%s

Mets à jour (true) si au moins un de ces cas : correctif de sécurité probable (openssl, openssh, glibc, sudo, systemd, curl, gnutls, nss, polkit, linux-firmware, noyau…) ; noyau linux-cachyos ; pilotes graphiques (mesa, vulkan-radeon, lib32-mesa, linux-firmware-amdgpu, ROCm) ; jeux (steam, gamescope, pipewire, wireplumber) ; lemonade-server ; ou image de plus de 21 jours avec plusieurs mises à jour. Sinon (seulement des paquets mineurs, rien d'important), réponds false.

Réponds UNIQUEMENT par un objet JSON, sans texte autour :
{"mettre_a_jour": true|false, "raison": "une ou deux phrases en français", "paquets_importants": ["nom ancienne → nouvelle", …]}`, versionImage, age, count, paquets)
}

// gardeJournal : au-delà de 2000 lignes, on ne garde que les 1500 dernières.
func gardeJournal(lignes []string) []string {
	if len(lignes) > 2000 {
		return lignes[len(lignes)-1500:]
	}
	return lignes
}

// profilDroits lit les droits déclarés dans profiledef.sh :
// ["/chemin"]="uid:gid:mode". Renvoie chemin → mode.
var motifDroits = regexp.MustCompile(`(?m)^\s*\["([^"]+)"\]="[0-9]+:[0-9]+:([0-7]+)"`)

func profilDroits(profiledef string) map[string]string {
	res := map[string]string{}
	for _, m := range motifDroits.FindAllStringSubmatch(profiledef, -1) {
		if strings.HasSuffix(m[1], "/") {
			continue
		}
		res[m[1]] = m[2]
	}
	return res
}
