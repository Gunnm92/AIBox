// Heure et date, avec une barre qui se remplit au fil des secondes.
import QtQuick
import Quickshell

Card {
    id: card
    width: 520 * u
    height: 232 * u

    SystemClock {
        id: clock
        precision: SystemClock.Seconds
    }

    Column {
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter
        spacing: 4 * card.u

        Text {
            text: Qt.formatTime(clock.date, "HH:mm")
            color: Theme.text
            font.family: "DejaVu Sans"
            font.pixelSize: 104 * card.u
            font.weight: Font.DemiBold
        }
        Text {
            text: clock.date.toLocaleDateString(Qt.locale("fr_FR"), "dddd d MMMM")
            color: Theme.dim
            font.family: "DejaVu Sans"
            font.pixelSize: 26 * card.u
        }
    }

    Rectangle {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: 6 * card.u
        radius: height / 2
        color: Theme.track

        Rectangle {
            width: parent.width * clock.date.getSeconds() / 59
            height: parent.height
            radius: height / 2
            gradient: Gradient {
                orientation: Gradient.Horizontal
                GradientStop { position: 0.0; color: Theme.accent }
                GradientStop { position: 1.0; color: Theme.accent }
            }
        }
    }
}
