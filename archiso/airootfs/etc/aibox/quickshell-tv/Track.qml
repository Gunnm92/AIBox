// Barre fine (6 px de maquette) : piste blanche translucide, remplissage en dégradé
// accent → second accent, de « from » à « to » (fractions de 0 à 1).
import QtQuick

Rectangle {
    id: track
    property real u: 1
    property real from: 0
    property real to: 0
    property alias fill: bar
    height: 6 * u
    radius: height / 2
    color: Theme.trackWhite
    clip: true

    Rectangle {
        id: bar
        x: track.width * Math.max(0, Math.min(1, track.from))
        width: track.width * Math.max(0, Math.min(1, track.to) - Math.max(0, track.from))
        height: parent.height
        radius: height / 2
        gradient: Gradient {
            orientation: Gradient.Horizontal
            GradientStop { position: 0.0; color: Theme.accent }
            GradientStop { position: 1.0; color: Theme.accent2 }
        }
    }
}
