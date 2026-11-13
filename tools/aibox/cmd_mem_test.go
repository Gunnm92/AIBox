package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// arbre écrit un faux système (proc, sys, run, montages) sous une racine temporaire.
func arbre(t *testing.T, fichiers map[string]string, dossiers []string) env {
	t.Helper()
	racine := t.TempDir()
	for _, d := range dossiers {
		if err := os.MkdirAll(filepath.Join(racine, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for rel, contenu := range fichiers {
		p := filepath.Join(racine, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(contenu), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return env{
		racine: racine,
		lancer: func(nom string, args ...string) (string, error) {
			switch nom {
			case "zramctl":
				if len(args) > 0 {
					return "zram0 4G lz4\n", nil
				}
				return "NAME ALGORITHM\nzram0 4G lz4\n", nil
			case "systemd-analyze":
				return "Startup finished in 5s (firmware) + 9s (loader)\nautre ligne\n", nil
			}
			return "", os.ErrNotExist
		},
	}
}

// fauxSysteme : un système avec image en RAM, données montées et GPU UMA trop grand.
func fauxSysteme(t *testing.T) env {
	t.Helper()
	return arbre(t, map[string]string{
		"proc/meminfo": "MemTotal:       16777216 kB\n" +
			"MemAvailable:   12582912 kB\n" +
			"Shmem:          8388608 kB\n" +
			"Cached:         9437184 kB\n",
		"sys/module/ttm/parameters/pages_limit":     "7340032\n",
		"sys/module/ttm/parameters/page_pool_size":  "4194304\n",
		"sys/class/drm/card0/device/mem_info_gtt_total": "68719476736\n",
		"sys/class/drm/card0/device/mem_info_gtt_used":  "1073741824\n",
		"sys/class/drm/card0/device/mem_info_vram_total": "2147483648\n",
		"run/archiso/copytoram/filesystem.squashfs":      strings.Repeat("x", 3*1024*1024),
		"run/aibox/data-present":                         "",
		"proc/self/mountinfo": "36 35 0:31 / /home rw - btrfs /dev/nvme0n1p2[/@data] rw\n" +
			"37 35 0:32 / /run/archiso/bootmnt rw - iso9660 /dev/sr0 ro\n" +
			"38 35 0:33 / /tmp rw - tmpfs tmpfs rw\n",
	}, []string{"home", "tmp", "run/archiso/cowspace"})
}

func TestMemSysteme(t *testing.T) {
	var b bytes.Buffer
	afficherMem(fauxSysteme(t), &b)
	sortie := b.String()

	attendus := []string{
		"Total",
		"16384",                   // MemTotal en Mio
		"DÉPASSÉ",                 // Shmem 8192 Mio > budget 6144
		"Attention : UMA > 1 Gio", // VRAM 2048 Mio
		"AIBOXDATA monté (/dev/nvme0n1p2)",
		"filesystem.squashfs  3M", // image en RAM
		"Encore montée sur /run/archiso/bootmnt",
		"zram0 4G lz4",
		"Startup finished in 5s",
		"Plafond GTT (ttm.pages_limit)",
		"28672", // pages_limit / 256
	}
	for _, s := range attendus {
		if !strings.Contains(sortie, s) {
			t.Errorf("la sortie ne contient pas %q\n%s", s, sortie)
		}
	}
}

func TestMemModeEphemere(t *testing.T) {
	e := arbre(t, map[string]string{
		"proc/meminfo":        "MemTotal: 1048576 kB\nShmem: 1024 kB\nCached: 2048 kB\n",
		"proc/self/mountinfo": "",
	}, []string{"tmp"})
	// Pas de zram sur ce système : zramctl échoue.
	e.lancer = func(nom string, args ...string) (string, error) {
		return "", os.ErrNotExist
	}
	var b bytes.Buffer
	afficherMem(e, &b)
	sortie := b.String()

	for _, s := range []string{
		"MODE ÉPHÉMÈRE",
		"Budget système (≤ 6144 Mio) : OK",
		"Pas de copie en RAM",
		"Libérée : la clé peut être retirée.",
		"aucun",
	} {
		if !strings.Contains(sortie, s) {
			t.Errorf("la sortie ne contient pas %q\n%s", s, sortie)
		}
	}
}

func TestMemArgumentsRefuses(t *testing.T) {
	var sortie, erreurs bytes.Buffer
	if code := cmdMem([]string{"en-trop"}, &sortie, &erreurs); code != 2 {
		t.Errorf("code = %d, attendu 2", code)
	}
}

func TestHumain(t *testing.T) {
	cas := map[uint64]string{
		512:                "512",
		1024:               "1,0K",
		10 * 1024:          "10K",
		1536 * 1024 * 1024: "1,5G",
		uint64(2) << 40: "2,0T",
	}
	for n, attendu := range cas {
		if got := humain(n); got != attendu {
			t.Errorf("humain(%d) = %q, attendu %q", n, got, attendu)
		}
	}
}
