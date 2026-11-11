// Carte translucide commune aux widgets de la TV : fond sombre, bord fin,
// reflet qui défile sur le haut, et apparition en fondu.
import QtQuick

Rectangle {
    id: card
    property real u: 1
    default property alias content: inner.data

    radius: 30 * u
    color: Theme.cardFill
    border.color: Theme.cardBorder
    border.width: Math.max(1, u)
    opacity: 0

    NumberAnimation on opacity {
        to: 1
        duration: 900
        easing.type: Easing.OutCubic
        running: true
    }

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

    // Filet lumineux qui fait le tour du haut de la carte.
    Rectangle {
        id: shimmer
        width: card.width * 0.3
        height: Math.max(2, 2 * card.u)
        y: 0
        radius: height / 2
        gradient: Gradient {
            orientation: Gradient.Horizontal
            GradientStop { position: 0.0; color: "#00E5FF00" }
            GradientStop { position: 0.5; color: Theme.accent }
            GradientStop { position: 1.0; color: "#00E5FF00" }
        }
        NumberAnimation on x {
            from: -shimmer.width
            to: card.width
            duration: 4200
            loops: Animation.Infinite
            running: true
        }
    }

    Item {
        id: inner
        anchors.fill: parent
        anchors.margins: 30 * card.u
    }
}
