// Machine : Lemonade (débit, état, modèles) et requêtes en haut, RAM (anneau),
// GPU et services en bas. Carte 660×360 de maquette, grille de 2 colonnes.
// Lemonade garde son jaune ; au repos, rien ne bouge. Une valeur qui change
// passe en bleu (FlashText) ; l'anneau de RAM glisse en 1 s quand elle change.
import QtQuick

Card {
    id: card
    property var lemonade: null
    property var sys: ({})
    width: 660 * u
    height: 360 * u

    readonly property bool online: lemonade !== null && lemonade !== undefined
    readonly property int active: online ? lemonade.active : 0
    readonly property real ram: sys.ram !== undefined ? sys.ram : 0
    readonly property real gpu: sys.gpu_temp !== null && sys.gpu_temp !== undefined ? sys.gpu_temp : -1
    readonly property int services: sys.containers !== undefined ? sys.containers : 0

    readonly property real colW: (width - 2 * padX * u - 30 * u) / 2
    readonly property real gridTop: 40 * u
    readonly property real row2: gridTop + 152 * u

    // 72276 → « 72 k », 1520000 → « 1,5 M » (espaces fines).
    function short(n) {
        if (n >= 1e6) return (n / 1e6).toFixed(1).replace(".", ",") + " M";
        if (n >= 1e4) return Math.round(n / 1000) + " k";
        return String(n).replace(/\B(?=(\d{3})+(?!\d))/g, " ");
    }

    // Place un texte par sa ligne de base (cotes de maquette dans la cellule).
    component Line: FlashText {
        property real base: 0
        y: base - baselineOffset
    }

    Text {
        text: "MACHINE"
        color: Theme.dim
        font.family: "DejaVu Sans"
        font.pixelSize: 22 * card.u
        font.letterSpacing: 3 * card.u
    }

    // Lemonade
    Item {
        y: card.gridTop
        width: card.colW
        Row {
            spacing: 10 * card.u
            height: 24 * card.u
            Rectangle {
                width: 12 * card.u
                height: width
                radius: width / 2
                color: card.online ? Theme.lemon : Theme.dim
                anchors.verticalCenter: parent.verticalCenter
            }
            Text {
                text: "LEMONADE"
                color: Theme.lemon
                font.family: "DejaVu Sans"
                font.pixelSize: 20 * card.u
                font.bold: true
                anchors.verticalCenter: parent.verticalCenter
            }
            FlashText {
                text: !card.online ? "hors ligne" : (card.active > 0 ? card.active + " en cours" : "au repos")
                baseColor: Theme.dim
                font.pixelSize: 20 * card.u
                anchors.verticalCenter: parent.verticalCenter
            }
        }
        Row {
            y: 92 * card.u - tps.baselineOffset
            spacing: 12 * card.u
            FlashText {
                id: tps
                text: card.online ? Math.round(card.lemonade.tps) : "–"
                font.pixelSize: 64 * card.u
                font.bold: true
            }
            Text {
                y: tps.baselineOffset - baselineOffset
                text: "tokens/s"
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 22 * card.u
            }
        }
        Line {
            base: 124 * card.u
            width: card.colW
            elide: Text.ElideRight
            text: !card.online ? "serveur hors ligne"
                : (card.lemonade.loaded && card.lemonade.loaded.length ? card.lemonade.loaded.join(", ") : "aucun modèle chargé")
            baseColor: Theme.dim
            font.pixelSize: 22 * card.u
        }
    }

    // Requêtes
    Item {
        x: card.colW + 30 * card.u
        y: card.gridTop
        width: card.colW
        Text {
            y: 18 * card.u - baselineOffset
            text: "REQUÊTES"
            color: Theme.dim
            font.family: "DejaVu Sans"
            font.pixelSize: 18 * card.u
            font.letterSpacing: 2.5 * card.u
        }
        Line {
            base: 92 * card.u
            text: card.online ? card.short(card.lemonade.requests) : "–"
            font.pixelSize: 64 * card.u
            font.bold: true
        }
        Line {
            base: 124 * card.u
            width: card.colW
            elide: Text.ElideRight
            text: card.online ? "entrée " + card.short(card.lemonade.tokens_in) + " · sortie " + card.short(card.lemonade.tokens_out) : ""
            baseColor: Theme.dim
            font.pixelSize: 20 * card.u
        }
    }

    // RAM : anneau de 112 px
    Item {
        y: card.row2
        width: card.colW
        height: 112 * card.u

        property real shown: 0
        Behavior on shown { NumberAnimation { duration: 1000; easing.type: Easing.OutCubic } }
        Component.onCompleted: shown = Qt.binding(() => card.ram)
        onShownChanged: ring.requestPaint()

        Canvas {
            id: ring
            width: 112 * card.u
            height: width
            onPaint: {
                var ctx = getContext("2d");
                ctx.reset();
                var c = width / 2, lw = width * 9 / 100, r = width * 42 / 100;
                ctx.lineWidth = lw;
                ctx.lineCap = "round";
                ctx.strokeStyle = "rgba(255,255,255,0.08)";
                ctx.beginPath();
                ctx.arc(c, c, r, 0, 2 * Math.PI);
                ctx.stroke();
                var f = Math.max(0, Math.min(1, parent.shown / 100));
                if (f > 0) {
                    ctx.strokeStyle = Theme.accent;
                    ctx.beginPath();
                    ctx.arc(c, c, r, -Math.PI / 2, -Math.PI / 2 + 2 * Math.PI * f);
                    ctx.stroke();
                }
            }
        }
        Column {
            x: ring.width + 18 * card.u
            anchors.verticalCenter: parent.verticalCenter
            FlashText {
                text: Math.round(card.ram) + " %"
                font.pixelSize: 40 * card.u
                font.bold: true
            }
            Text {
                text: "RAM"
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 20 * card.u
            }
        }
    }

    // GPU et services
    Item {
        x: card.colW + 30 * card.u
        y: card.row2
        width: card.colW
        height: 112 * card.u

        Item {
            width: parent.width
            height: 26 * card.u
            Text {
                text: "GPU"
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 22 * card.u
            }
            FlashText {
                anchors.right: parent.right
                text: card.gpu >= 0 ? card.gpu + " °C" : "–"
                font.pixelSize: 22 * card.u
                font.bold: true
            }
        }
        Track {
            u: card.u
            y: 36 * card.u
            width: parent.width
            to: card.gpu >= 0 ? card.gpu / 100 : 0
        }
        Item {
            y: 62 * card.u
            width: parent.width
            height: 26 * card.u
            Text {
                text: "Services"
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 22 * card.u
            }
            FlashText {
                anchors.right: parent.right
                text: card.services + (card.services > 1 ? " actifs" : " actif")
                font.pixelSize: 22 * card.u
                font.bold: true
            }
        }
        Row {
            y: 98 * card.u
            spacing: 9 * card.u
            Repeater {
                model: Math.min(card.services, 13)
                Rectangle {
                    width: 12 * card.u
                    height: width
                    radius: width / 2
                    color: Theme.accent
                }
            }
        }
    }
}
