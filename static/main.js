let currentLocation = new URL(window.location)

let scene = "XVlBzg"  // todo: user input for scene value
let socket = new WebSocket("ws://" + currentLocation.host + "/api/v1/" + scene + "/listen")

function getSceneStorageKey(scene) {
    return "scene:" + scene
}

function loadObjects(scene) {
    let value = localStorage.getItem(getSceneStorageKey(scene))
    if (value == null) {
        let objects = {}
        storeObjects(scene, objects)
        return objects
    }
    return JSON.parse(value)
}

function storeObjects(scene, objects) {
    try {
        localStorage.setItem(getSceneStorageKey(scene), JSON.stringify(objects))
    } catch (e) {
        if (e instanceof DOMException) {
            alert("Exceeded memory quota on local storage")  // todo: show this message more nicely
        }
        console.log(e)
    }
}

let canvas = document.getElementById("canvas")
canvas.width = 600
canvas.height = 600
let context = canvas.getContext("2d")   // todo: WebGL
context.scale(canvas.width / 2, -canvas.height / 2)
context.translate(1, -1)
let objects = loadObjects(scene)

function drawObjects(context, objects) {
    context.clearRect(-1, 1, 1, 1)
    context.lineWidth = 0.0025
    for (let id in objects) {
        let object = objects[id]
        switch (object["type"]) {
            case "line":
                let begin = object["content"]["begin"]
                let end = object["content"]["end"]
                context.beginPath()
                context.moveTo(begin["x"], begin["y"])
                context.lineTo(end["x"], end["y"])
                context.stroke()
                break
        }
    }
}

drawObjects(context, objects)

socket.onclose = function (event) {
    // todo: try to reconnect
    alert("connection to the server is lost, scene updates won't be received")
    console.log(event)
}

socket.onmessage = function (event) {
    let object = JSON.parse(event.data)
    switch (object["type"]) {
        case "clear":
            objects = {}
            break
        case "line":
            objects[object["id"]] = object
            break
    }
    storeObjects(scene, objects)
    drawObjects(context, objects)
}

socket.onopen = function (event) {
    objects = {}
    storeObjects(scene, objects)
    console.log(event)
}
