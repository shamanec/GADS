

export default function StreamCanvas({ deviceData }) {
    let screenDimensions = deviceData.screen_size.split("x")
    let screenX = parseInt(screenDimensions[0])
    let screenY = parseInt(screenDimensions[1])

    let screen_ratio = screenX / screenY

    let canvasHeight = "850px"
    let canvasWidth = (850 * screen_ratio) + 'px'

    return (
        <div id="stream-div">
            <Canvas canvasWidth={canvasWidth} canvasHeight={canvasHeight}></Canvas>
            <Stream canvasWidth={canvasWidth} canvasHeight={canvasHeight}></Stream>
        </div>
    )
}

function Canvas({ canvasWidth, canvasHeight }) {
    return (
        <canvas id="actions-canvas" style={{ position: "absolute", width: canvasWidth, height: canvasHeight }}></canvas>
    )
}

function Stream({ canvasWidth, canvasHeight }) {
    return (
        <img id="image-stream" style={{ width: canvasWidth, height: canvasHeight }}></img>
    )
}