import TdaCodec, { ButtonState, MessageType } from './codec.js'
var stream;
var canvas;
var codec;

function connectStream() {
    codec = new TdaCodec()
    if (stream) { stream.close(); }

    var ctx = canvas.getContext("2d");

    stream = new WebSocket("ws://"+document.location.host+"/connect");
    stream.onerror = function(event) { console.log("ws error:", event);
    }
    stream.onopen = function(event) {
        console.log("ws opened");
    }
    stream.onclose = function(event) {
        console.log("ws closed");
    }
    stream.onmessage = function(event) {
        const blob = event.data
        codec.decMessageType(blob).then(messageType => {
            if (messageType === MessageType.PNG_FRAME) {
                codec.decRegion(blob).then(region => {
                    if (ctx.canvas.width < region.right) {
                        ctx.canvas.width = region.right;
                    }
                    if (ctx.canvas.height < region.bottom) {
                        ctx.canvas.height = region.bottom;
                    }
                    codec.decPng(blob).then(bitmap => {
                        ctx.drawImage(bitmap, region.left, region.top);
                    })
                })
            } else {
                throw new Error(`unexpected message type ${messageType}`);
            }
        }).catch(err => console.error(err))
    }
}

export function handleMouseEnter(e) {
    // Grab focus on canvas to catch keyboard events.
    canvas.focus();
}

export function handleMouseLeave(e) {
    canvas.blur();
}

export function handleContextMenu(e) {
    // Prevent native context menu to not obscure remote context menu.
    return false;
}

export function handleMouseMove(e) {
    if (stream === undefined) {
        return;
    }
    var rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left);
    const y = (e.clientY - rect.top);
    let buffer = codec.encMouseMove(x, y)
    stream.send(buffer);
}

export function handleMouseDown(e) {
    handleMouseButton(e.button, ButtonState.DOWN)
}

export function handleMouseUp(e) {
    handleMouseButton(e.button, ButtonState.UP)
}

function handleMouseButton(button, state) {
    if (stream === undefined) {
        return;
    }
    let buffer = codec.encMouseButton(button, state)
    stream.send(buffer);
}

export function handleKeyDown(e) {
    handleKeyboardButton(e.code, ButtonState.DOWN);
    return false;
}

export function handleKeyUp(e) {
    handleKeyboardButton(e.code, ButtonState.UP);
    return false;
}

function handleKeyboardButton(code, state) {
    if (stream === undefined) {
        return;
    }
    let buffer = codec.encKeyboard(code, state)
    if (buffer === null) {
        return
    }
    stream.send(buffer);
}

window.onload = function() {
    canvas = document.getElementById("canvas");
    // Allow catching of keyboard events.
    canvas.tabIndex = 1000;
    canvas.onmouseenter = handleMouseEnter;
    canvas.onmouseleave = handleMouseLeave;
    canvas.oncontextmenu = handleContextMenu;
    canvas.onmousemove = handleMouseMove;
    canvas.onmousedown = handleMouseDown;
    canvas.onmouseup = handleMouseUp;
    canvas.onkeydown = handleKeyDown;
    canvas.onkeyup = handleKeyUp;

    document.getElementById("button-start").addEventListener('click', connectStream);
}