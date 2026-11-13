// aibox : point d'entrée unique des commandes d'aibox.
//
//	aibox                 aide (même chose que --help)
//	aibox <commande> ...  lance la commande, arguments transmis tels quels
//
// Le binaire ne fait que l'aide et l'aiguillage : la logique reste dans les
// scripts aibox-* (voir docs/commandes.md). Compilé dans un conteneur Go par
// tools/aibox/build.sh, puis rangé dans l'image.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// fonctionCommande : une commande implémentée dans le binaire. Reçoit les
// arguments qui suivent le nom, et renvoie le code de sortie.
type fonctionCommande func(args []string, sortie, erreurs io.Writer) int

// commande : une entrée de l'aide et de l'aiguillage.
type commande struct {
	nom      string           // nom tapé après « aibox »
	exe      string           // script lancé si fonction est nil (PATH, ou /usr/local/sbin)
	fonction fonctionCommande // logique en Go ; nil tant que la commande reste un script
	groupe   string           // rubrique de l'aide
	resume   string           // une ligne
	usages   []string         // exemples d'usage, affichés sous la ligne
}

// groupes : ordre d'affichage de l'aide.
var groupes = []string{"Mise à jour", "Usage courant", "Administration"}

var commandes = []commande{
	{
		nom: "update", fonction: cmdUpdate, groupe: "Mise à jour",
		resume: "Mise à jour du système : image décidée par le LLM local, ou nouveaux commits",
		usages: []string{
			"aibox update [--dry-run] [--no-install] [--force] [--pkglist FICHIER]",
			"aibox update --podman [--dry-run]   conteneurs Podman seulement",
		},
	},
	{
		nom: "build", fonction: cmdBuild, groupe: "Mise à jour",
		resume: "Construit l'image depuis GitHub (origin/main), sans l'installer",
		usages: []string{"sudo aibox build [--ref BRANCHE] [--fetch-only]"},
	},
	{
		nom: "boot", fonction: cmdBoot, groupe: "Mise à jour",
		resume: "Installe une image sur AIBOXBOOT, sans toucher aux données",
		usages: []string{"sudo aibox boot --iso FICHIER"},
	},
	{
		nom: "tv-mode", fonction: cmdTvMode, groupe: "Usage courant",
		resume: "Mode de la session TV : lanceur (par défaut), gamescope ou bureau",
		usages: []string{"aibox tv-mode [--once] [gamescope|hyprland|bureau]"},
	},
	{
		nom: "nas", fonction: cmdNas, groupe: "Usage courant",
		resume: "Partages réseau (SMB) montés à la demande",
		usages: []string{"aibox nas list", "sudo aibox nas add //hôte/partage /mnt/X", "sudo aibox nas remove /mnt/X"},
	},
	{
		nom: "tv-run", fonction: cmdTvRun, groupe: "Usage courant",
		resume: "Lance une commande sur l'écran de la TV (la session Hyprland doit tourner)",
		usages: []string{`aibox tv-run env WSQUASHFS_HIDRAW=1 wsquashfs-launcher "Jeu.wsquashfs"`},
	},
	{
		nom: "run-exe", fonction: cmdRunExe, groupe: "Usage courant",
		resume: "Lance un programme Windows (.exe, .msi) avec un Proton installé",
		usages: []string{"aibox run-exe prog.exe [args]"},
	},
	{
		nom: "wsquashfs", exe: "wsquashfs-launcher", groupe: "Usage courant",
		resume: "Lance un jeu Windows au format Batocera (.wsquashfs)",
		usages: []string{`aibox wsquashfs "Jeu.wsquashfs"`, "aibox wsquashfs --info FICHIER"},
	},
	{
		nom: "keyboard", fonction: cmdKeyboard, groupe: "Usage courant",
		resume: "Disposition du clavier du bureau d'administration (noVNC)",
		usages: []string{"aibox keyboard [mac|pc|<xkb> [variante]]"},
	},
	{
		nom: "resolution", fonction: cmdResolution, groupe: "Usage courant",
		resume: "Taille de l'écran virtuel du bureau d'administration",
		usages: []string{"aibox resolution [1920x1080[@60]|client]"},
	},
	{
		nom: "mem", fonction: cmdMem, groupe: "Usage courant",
		resume: "État de la mémoire, du GPU, de l'image en RAM, de la clé et du zram",
		usages: []string{"aibox mem"},
	},
	{
		nom: "diag", fonction: cmdDiag, groupe: "Usage courant",
		resume: "Rapport de diagnostic complet, enregistré sur la clé ou AIBOXBOOT",
		usages: []string{"sudo aibox diag"},
	},
	{
		nom: "setup-disk", exe: "aibox-setup-disk", groupe: "Administration",
		resume: "EFFACE un disque et le prépare (AIBOXBOOT + AIBOXDATA). Une seule fois",
		usages: []string{"sudo aibox setup-disk /dev/nvmeXnY [--swap 16G] [--iso F]"},
	},
}

// dossiers : où chercher un script si le PATH ne le trouve pas (sbin pour root).
var dossiers = []string{"/usr/local/bin", "/usr/local/sbin"}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run aiguille les arguments. Ne revient que pour l'aide ou en cas d'erreur :
// une commande lancée remplace le processus (syscall.Exec).
func run(args []string, sortie, erreurs io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		ecrireAide(sortie)
		return 0
	}
	c, ok := trouver(args[0])
	if !ok {
		fmt.Fprintf(erreurs, "aibox : commande inconnue « %s ». Liste : aibox --help\n", args[0])
		return 2
	}
	if c.fonction != nil {
		return c.fonction(args[1:], sortie, erreurs)
	}
	chemin, err := chercher(c.exe)
	if err != nil {
		fmt.Fprintf(erreurs, "aibox : %s introuvable : %v\n", c.exe, err)
		return 127
	}
	argv := append([]string{c.exe}, args[1:]...)
	if err := syscall.Exec(chemin, argv, os.Environ()); err != nil {
		fmt.Fprintf(erreurs, "aibox : impossible de lancer %s : %v\n", c.exe, err)
		return 126
	}
	return 0
}

// trouver renvoie la commande de ce nom.
func trouver(nom string) (commande, bool) {
	for _, c := range commandes {
		if c.nom == nom {
			return c, true
		}
	}
	return commande{}, false
}

// chercher renvoie le chemin du script : le PATH, puis les dossiers de dossiers.
func chercher(exe string) (string, error) {
	if p, err := exec.LookPath(exe); err == nil {
		return p, nil
	}
	for _, d := range dossiers {
		p := filepath.Join(d, exe)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("absent du PATH et de %s", strings.Join(dossiers, ", "))
}

// ecrireAide affiche l'aide, groupée par rubrique.
func ecrireAide(w io.Writer) {
	fmt.Fprintln(w, "aibox : commandes du système (détail : docs/commandes.md du dépôt).")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Usage : aibox <commande> [options]")
	fmt.Fprintln(w, "  La commande s'écrit après « aibox » : « aibox update », « aibox mem »… ;")
	fmt.Fprintln(w, "  les options (--dry-run, --podman…) et les valeurs (FICHIER, BRANCHE…) viennent après.")
	fmt.Fprintln(w, "  Aide : aibox --help")
	fmt.Fprintln(w, "  Chaque commande lance un script aibox-* (indiqué sous son nom) ; ces scripts")
	fmt.Fprintln(w, "  marchent aussi directement, par exemple « aibox-auto-update --podman ».")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Commandes, par rubrique (la première colonne est le nom à taper) :")
	for _, g := range groupes {
		fmt.Fprintf(w, "\n%s\n", g)
		for _, c := range commandes {
			if c.groupe != g {
				continue
			}
			fmt.Fprintf(w, "  %-12s %s\n", c.nom, c.resume)
			if c.fonction == nil {
				fmt.Fprintf(w, "  %-12s   script : %s\n", "", c.exe)
			}
			for _, u := range c.usages {
				fmt.Fprintf(w, "  %-12s   exemple : %s\n", "", u)
			}
		}
	}
}
