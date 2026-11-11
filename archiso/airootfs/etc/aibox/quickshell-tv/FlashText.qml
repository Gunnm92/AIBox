// Texte de valeur : passe en bleu (accent) quand la valeur change, puis revient
// à sa couleur en 600 ms. Rien au premier affichage ni si la valeur ne bouge pas.
import QtQuick

Text {
    id: t
    property color baseColor: Theme.text
    property bool ready: false
    color: baseColor
    font.family: "DejaVu Sans"

    onTextChanged: {
        if (ready) flash.restart();
        ready = true;
    }
    ColorAnimation {
        id: flash
        target: t
        property: "color"
        from: Theme.accent
        to: t.baseColor
        duration: 600
        easing.type: Easing.OutCubic
    }
}
