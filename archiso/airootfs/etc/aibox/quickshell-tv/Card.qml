// Carte translucide commune aux widgets de la TV : fond sombre, bord fin, reflet en haut.
import QtQuick

Rectangle {
    id: card
    property real u: 1
    default property alias content: inner.data

    radius: 30 * u
    color: Theme.cardFill
    border.color: Theme.cardBorder
    border.width: Math.max(1, u)

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.top: parent.top
        height: parent.height * 0.45
        radius: card.radius
        gradient: Gradient {
            GradientStop { position: 0.0; color: Theme.sheen }
            GradientStop { position: 1.0; color: "#00FFFFFF" }
        }
    }

    Item {
        id: inner
        anchors.fill: parent
        anchors.margins: 30 * card.u
    }
}
