package main

import (
	"os/user"
	"testing"
)

// TestDossierPersonnel : sous sudo (root, SUDO_USER renseigné), le home de
// l'utilisateur d'origine ; sinon $HOME tel quel.
func TestDossierPersonnel(t *testing.T) {
	actuel, err := user.Current()
	if err != nil {
		t.Skip("utilisateur courant introuvable :", err)
	}
	if got := dossierPersonnel("/home/x", "", 1000); got != "/home/x" {
		t.Errorf("sans sudo : %q, attendu /home/x", got)
	}
	if got := dossierPersonnel("/root", "", 0); got != "/root" {
		t.Errorf("root sans SUDO_USER : %q, attendu /root", got)
	}
	if got := dossierPersonnel("/root", "root", 0); got != "/root" {
		t.Errorf("SUDO_USER=root : %q, attendu /root", got)
	}
	if got := dossierPersonnel("/root", "utilisateur-inexistant-aibox", 0); got != "/root" {
		t.Errorf("SUDO_USER inconnu : %q, attendu /root", got)
	}
	if got := dossierPersonnel("/root", actuel.Username, 0); got != actuel.HomeDir {
		t.Errorf("sudo de %s : %q, attendu %q", actuel.Username, got, actuel.HomeDir)
	}
}
