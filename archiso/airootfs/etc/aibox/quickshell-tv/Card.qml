// Carte translucide commune aux widgets de la TV (maquette « Direction 2 v3 ») :
// fond sombre à 66 %, bord blanc à 8 %, filet clair en haut, rayon 32. Le flou
// du fond (verre dépoli) vient de Hyprland (layer_rule de hypr-tv). Pas
// d'animation décorative : seulement un fondu à l'apparition.
import QtQuick

Rectangle {
    id: card
    property real u: 1
    property real padX: 36
    property real padY: 26
    default property alias content: inner.data

    radius: 32 * u
    color: Theme.cardFill
    border.color: Theme.cardBorder
    border.width: Math.max(1, u)
    opacity: 0

    NumberAnimation on opacity {
        to: 1
        duration: 600
        easing.type: Easing.OutCubic
        running: true
    }

    // Filet clair sur le bord haut (inset 0 1px rgba(255,255,255,.06)).
    Rectangle {
        x: card.radius
        width: card.width - 2 * card.radius
        y: Math.max(1, card.u)
        height: Math.max(1, card.u)
        color: "#0FFFFFFF"
    }

    Item {
        id: inner
        anchors.fill: parent
        anchors.leftMargin: card.padX * card.u
        anchors.rightMargin: card.padX * card.u
        anchors.topMargin: card.padY * card.u
        anchors.bottomMargin: card.padY * card.u
    }
}
