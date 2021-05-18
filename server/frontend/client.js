const {PollRequest, PollResponse} = require('./service_pb.js')
const {DrawerClient} = require('./service_grpc_web_pb.js')

var url = new URL(document.location)
var client = new DrawerClient(url.protocol + '//' + url.hostname + ':' + (Number(url.port) + 1).toString())
let context
var maxDrawingTimestamp = 0

function Main() {
    if (window.location.hash) {
        BeginPolling(window.location.hash.slice(1))
    } else {
        document.getElementById("SubmitButton").addEventListener("click", function(e) {
            SceneId = document.getElementById("SceneIdInput").value
            BeginPolling(SceneId)
        })
    }
}

function HideSceneIdForm() {
    document.getElementById("SceneSelector").style.display = 'none'
}

function BeginPolling(SceneId) {
    console.log('Starting polling with SceneId', SceneId)
    window.location.hash = SceneId
    HideSceneIdForm()

    let canvas = document.getElementById("canvas")
    canvas.width = 800
    canvas.height = 800
    context = canvas.getContext("2d")   // todo: WebGL
    context.scale(canvas.width / 2, -canvas.height / 2)
    context.translate(1, -1)
    context.clearRect(-1, 1, 1, 1)
    context.lineWidth = 0.0025

    setInterval(function() {
        PollAndDraw(SceneId)
    }, 1000)
}

function PollAndDraw(SceneId) {
    let req = new PollRequest()
    req.setSceneId(SceneId)
    req.setAuthenticator('')
    req.setAfterTimestamp(maxDrawingTimestamp)

    client.poll(req, {}, function (err, pollResult) {
        if (err) {
            console.log('Got error: ', err)
        }
        
        let drawings = pollResult.getDrawingsList()
        if (drawings.length != 0) {
            maxDrawingTimestamp = pollResult.getLastTimestamp()
        }
        console.log('Received ', drawings.length, ' drawings, current maxDrawingTimestamp timestamp', maxDrawingTimestamp)
        for (const drawing of drawings) {
            Draw(drawing)
        }
    })
}

function Draw(drawing) {
    if (drawing.hasLine()) {
        let line = drawing.getLine()

        context.beginPath()
        context.moveTo(line.getFrom().getX(), line.getFrom().getY())
        context.lineTo(line.getTo().getX(), line.getTo().getY())
        context.stroke()
    } else if (drawing.hasRectangle()) {
        let rect = drawing.getRectangle()
        let lowerLeft = rect.getLowerLeft()
        let upperRight = rect.getUpperRight()

        context.strokeRect(lowerLeft.getX(), upperRight.getY(), upperRight.getX() - lowerLeft.getX(), lowerLeft.getY() - upperRight.getY())
    } else if (drawing.hasCircle()) {
        let circle = drawing.getCircle()
        
        context.beginPath()
        context.arc(circle.getCenter().getX(), circle.getCenter().getY(), circle.getRadius(), 0, 2 * Math.PI)
        context.stroke()
    }
}

window.onload = Main
