const messagesEl = document.getElementById("messages");
const inputEl = document.getElementById("message-input");
const sendBtn = document.getElementById("send-btn");
const usernameDisplay = document.getElementById("username-display");
const channelListEl = document.getElementById("channel-list");
const currentChannelTitleEl = document.getElementById("current-channel-title");
const voiceJoinBtn = document.getElementById("voice-join-btn");
const voiceLeaveBtn = document.getElementById("voice-leave-btn");
const voiceStatusEl = document.getElementById("voice-status");

let currentChannel = "general";
const username = "User" + Math.floor(Math.random() * 1000);
usernameDisplay.textContent = username;

function addMessage(text, author = "System") {
    const row = document.createElement("div");
    row.className = "message";
    row.innerHTML = `<strong>${author}</strong>: ${text}`;
    messagesEl.appendChild(row);
    messagesEl.scrollTop = messagesEl.scrollHeight;
}

function clearMessages() {
    messagesEl.innerHTML = "";
}

async function loadHistoryForCurrentChannel() {
    try {
        const res = await fetch(`/api/history?channel=${encodeURIComponent(currentChannel)}`);
        if (!res.ok) {
            throw new Error("failed to load history");
        }
        const data = await res.json();
        clearMessages();
        data.forEach((m) => addMessage(m.text, m.author));
    } catch (err) {
        console.error("history error", err);
        addMessage("Failed to load history");
    }
}

function setActiveChannel(newChannel) {
    if (!newChannel || newChannel === currentChannel) {
        return;
    }

    currentChannel = newChannel;
    currentChannelTitleEl.textContent = `#${currentChannel}`;
    document.querySelectorAll(".channel").forEach((el) => {
        el.classList.toggle("channel--active", el.dataset.channel === currentChannel);
    });
    loadHistoryForCurrentChannel();
}

channelListEl.addEventListener("click", (e) => {
    const item = e.target.closest(".channel");
    if (!item) {
        return;
    }
    setActiveChannel(item.dataset.channel);
});

const protocol = location.protocol === "https:" ? "wss" : "ws";
const socket = new WebSocket(`${protocol}://${location.host}/ws`);

socket.addEventListener("open", () => {
    addMessage("Connected to chat server");
    loadHistoryForCurrentChannel();
});

socket.addEventListener("close", () => {
    addMessage("Disconnected from server");
});

socket.addEventListener("message", (event) => {
    try {
        const msg = JSON.parse(event.data);
        if (msg.type === "chat" && msg.channel === currentChannel) {
            addMessage(msg.text, msg.author);
        }
    } catch (e) {
        console.error("invalid message", e);
    }
});

function sendMessage() {
    const text = inputEl.value.trim();
    if (!text) {
        return;
    }

    const msg = { author: username, text, channel: currentChannel };
    if (socket.readyState !== WebSocket.OPEN) {
        addMessage("Connection with server refused");
        return;
    }

    socket.send(JSON.stringify(msg));
    inputEl.value = "";
}

sendBtn.addEventListener("click", sendMessage);
inputEl.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
        sendMessage();
    }
});

// Very lightweight voice transport: sends MediaRecorder chunks over binary WS.
let voiceSocket = null;
let mediaRecorder = null;

async function joinVoice() {
    if (voiceSocket && voiceSocket.readyState === WebSocket.OPEN) {
        return;
    }

    try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: false });
        voiceSocket = new WebSocket(`${protocol}://${location.host}/ws/voice`);
        voiceSocket.binaryType = "arraybuffer";

        voiceSocket.addEventListener("open", () => {
            voiceStatusEl.textContent = "Voice: connected";
            voiceJoinBtn.disabled = true;
            voiceLeaveBtn.disabled = false;

            mediaRecorder = new MediaRecorder(stream, { mimeType: "audio/webm" });
            mediaRecorder.ondataavailable = (event) => {
                if (event.data && event.data.size > 0 && voiceSocket.readyState === WebSocket.OPEN) {
                    event.data.arrayBuffer().then((buf) => voiceSocket.send(buf));
                }
            };
            mediaRecorder.start(300);
        });

        voiceSocket.addEventListener("close", () => {
            voiceStatusEl.textContent = "Voice: disconnected";
            voiceJoinBtn.disabled = false;
            voiceLeaveBtn.disabled = true;
        });
    } catch (err) {
        console.error("voice error", err);
        voiceStatusEl.textContent = "Voice: permission error";
    }
}

function leaveVoice() {
    if (mediaRecorder && mediaRecorder.state !== "inactive") {
        mediaRecorder.stop();
    }
    mediaRecorder = null;

    if (voiceSocket) {
        voiceSocket.close();
    }
    voiceSocket = null;
    voiceStatusEl.textContent = "Voice: disconnected";
    voiceJoinBtn.disabled = false;
    voiceLeaveBtn.disabled = true;
}

voiceJoinBtn.addEventListener("click", joinVoice);
voiceLeaveBtn.addEventListener("click", leaveVoice);
