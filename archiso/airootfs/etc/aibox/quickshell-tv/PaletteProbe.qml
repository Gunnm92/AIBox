// Lit les pixels du fond d'écran du jour dans un canevas hors écran,
// en tire la teinte dominante et applique la palette au thème.
import QtQuick
import "palette.js" as Palette

Canvas {
    id: probe
    width: 48
    height: 27
    opacity: 0
    property string source: ""
    signal ready(real hue)

    onSourceChanged: {
        if (source !== "") loadImage(source);
    }
    onImageLoaded: requestPaint()

    onPaint: {
        if (source === "" || !isImageLoaded(source)) return;
        var ctx = getContext("2d");
        ctx.reset();
        ctx.drawImage(source, 0, 0, width, height);
        var data = ctx.getImageData(0, 0, width, height).data;
        probe.ready(Palette.dominantHue(data));
    }
}
