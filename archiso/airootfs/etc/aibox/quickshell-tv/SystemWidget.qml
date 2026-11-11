// État d'aibox : anneau de mémoire, température GPU et nombre de services.
import QtQuick

Card {
    id: card
    property var sys: ({})
    property string updated: ""
    width: 460 * u
    height: 210 * u

    readonly property real ram: sys.ram !== undefined ? sys.ram : 0
    readonly property real gpu: sys.gpu_temp !== null && sys.gpu_temp !== undefined ? sys.gpu_temp : -1
    readonly property real services: sys.containers !== undefined ? sys.containers : 0

    Canvas {
        id: ring
        width: 150 * card.u
        height: width
        anchors.left: parent.left
        anchors.verticalCenter: parent.verticalCenter

        onPaint: {
            var ctx = getContext("2d");
            ctx.reset();
            var c = width / 2;
            var r = c - 9 * card.u;
            ctx.lineWidth = 14 * card.u;
            ctx.lineCap = "round";
            ctx.strokeStyle = Theme.track;
            ctx.beginPath();
            ctx.arc(c, c, r, 0, 2 * Math.PI);
            ctx.stroke();
            ctx.strokeStyle = Theme.accent;
            ctx.beginPath();
            ctx.arc(c, c, r, -Math.PI / 2, -Math.PI / 2 + 2 * Math.PI * Math.min(1, card.ram / 100));
            ctx.stroke();
        }
        onWidthChanged: requestPaint()
        Connections {
            target: card
            function onRamChanged() { ring.requestPaint() }
        }

        Column {
            anchors.centerIn: parent
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: card.ram + "%"
                color: Theme.text
                font.family: "DejaVu Sans"
                font.pixelSize: 40 * card.u
                font.weight: Font.DemiBold
            }
            Text {
                anchors.horizontalCenter: parent.horizontalCenter
                text: "RAM"
                color: Theme.dim
                font.family: "DejaVu Sans"
                font.pixelSize: 18 * card.u
            }
        }
    }

    Column {
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        width: parent.width - ring.width - 30 * card.u
        spacing: 22 * card.u

        // GPU : de 30 °C (vide) à 90 °C (plein).
        Column {
            width: parent.width
            spacing: 8 * card.u
            Item {
                width: parent.width
                height: 30 * card.u
                Text {
                    text: "GPU"
                    color: Theme.dim
                    font.family: "DejaVu Sans"
                    font.pixelSize: 22 * card.u
                }
                Text {
                    anchors.right: parent.right
                    text: card.gpu >= 0 ? card.gpu + " °C" : "--"
                    color: Theme.text
                    font.family: "DejaVu Sans"
                    font.pixelSize: 22 * card.u
                    font.weight: Font.DemiBold
                }
            }
            Rectangle {
                width: parent.width
                height: 8 * card.u
                radius: height / 2
                color: Theme.track
                Rectangle {
                    width: parent.width * Math.max(0, Math.min(1, (card.gpu - 30) / 60))
                    height: parent.height
                    radius: height / 2
                    gradient: Gradient {
                        orientation: Gradient.Horizontal
                        GradientStop { position: 0.0; color: Theme.accent }
                        GradientStop { position: 1.0; color: Theme.accent2 }
                    }
                }
            }
        }

        // Services : une pastille par conteneur, jusqu'à vingt.
        Column {
            width: parent.width
            spacing: 8 * card.u
            Item {
                width: parent.width
                height: 30 * card.u
                Text {
                    text: "Services"
                    color: Theme.dim
                    font.family: "DejaVu Sans"
                    font.pixelSize: 22 * card.u
                }
                Text {
                    anchors.right: parent.right
                    text: card.services + " actifs"
                    color: Theme.text
                    font.family: "DejaVu Sans"
                    font.pixelSize: 22 * card.u
                    font.weight: Font.DemiBold
                }
            }
            Row {
                spacing: 5 * card.u
                Repeater {
                    model: Math.min(20, card.services)
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
}
