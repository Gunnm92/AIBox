// Heure (120 px), date (30 px) et barre qui se remplit au fil des secondes.
// Carte 660×200 de maquette. Les textes sont placés par leur ligne de base, pour
// que la barre reste sous la date.
import QtQuick
import Quickshell

Card {
    id: card
    width: 660 * u
    height: 200 * u
    padY: 28

    SystemClock {
        id: clock
        precision: SystemClock.Seconds
    }

    Text {
        id: time
        // Haut des chiffres au ras de la marge : ligne de base à 88 (hauteur des chiffres).
        y: 88 * card.u - baselineOffset
        text: Qt.formatTime(clock.date, "HH:mm")
        color: Theme.text
        font.family: "DejaVu Sans"
        font.pixelSize: 120 * card.u
        font.bold: true
        font.letterSpacing: -2 * card.u
    }
    Text {
        y: 128 * card.u - baselineOffset
        text: clock.date.toLocaleDateString(Qt.locale("fr_FR"), "dddd d MMMM")
        color: Theme.dim
        font.family: "DejaVu Sans"
        font.pixelSize: 30 * card.u
    }
    Track {
        u: card.u
        width: parent.width
        anchors.bottom: parent.bottom
        anchors.bottomMargin: -4 * card.u   // à 24 px du bas de la carte
        to: clock.date.getSeconds() / 59
    }
}
