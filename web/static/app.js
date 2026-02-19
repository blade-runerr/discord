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

// WebRTC voice: media is peer-to-peer, websocket is signaling only.
let voiceSocket = null;
let selfPeerId = "";
let localStream = null;
const peerConnections = new Map();
const remoteAudioEls = new Map();

const rtcConfig = {
    iceServers: [{ urls: "stun:stun.l.google.com:19302" }],
};

async function joinVoice() {
    if (voiceSocket && voiceSocket.readyState === WebSocket.OPEN) {
        return;
    }

    try {
        localStream = await navigator.mediaDevices.getUserMedia({
            audio: {
                echoCancellation: true,
                noiseSuppression: true,
                autoGainControl: true,
            },
            video: false,
        });
        voiceSocket = new WebSocket(`${protocol}://${location.host}/ws/voice`);

        voiceSocket.addEventListener("open", () => {
            voiceStatusEl.textContent = "Voice: connected";
            voiceJoinBtn.disabled = true;
            voiceLeaveBtn.disabled = false;
        });

        voiceSocket.addEventListener("message", async (event) => {
            let msg;
            try {
                msg = JSON.parse(event.data);
            } catch (err) {
                console.error("invalid signaling message", err);
                return;
            }

            if (msg.type === "welcome") {
                selfPeerId = msg.id || "";
                const peers = Array.isArray(msg.peers) ? msg.peers : [];
                for (const peerId of peers) {
                    await makeOffer(peerId);
                }
                return;
            }

            if (msg.type === "peer_left") {
                closePeer(msg.id);
                return;
            }

            if (!msg.from || msg.from === selfPeerId) {
                return;
            }

            if (msg.type === "offer") {
                await handleOffer(msg.from, msg.sdp);
            } else if (msg.type === "answer") {
                await handleAnswer(msg.from, msg.sdp);
            } else if (msg.type === "candidate") {
                await handleCandidate(msg.from, msg.candidate);
            }
        });

        voiceSocket.addEventListener("close", () => {
            cleanupVoiceState();
        });
    } catch (err) {
        console.error("voice error", err);
        voiceStatusEl.textContent = "Voice: permission error";
    }
}

function leaveVoice() {
    if (voiceSocket) {
        voiceSocket.close();
    }
    cleanupVoiceState();
}

function cleanupVoiceState() {
    for (const peerId of peerConnections.keys()) {
        closePeer(peerId);
    }

    if (localStream) {
        localStream.getTracks().forEach((track) => track.stop());
    }

    localStream = null;
    selfPeerId = "";
    voiceSocket = null;
    voiceStatusEl.textContent = "Voice: disconnected";
    voiceJoinBtn.disabled = false;
    voiceLeaveBtn.disabled = true;
}

function createPeerConnection(peerId) {
    let pc = peerConnections.get(peerId);
    if (pc) {
        return pc;
    }

    pc = new RTCPeerConnection(rtcConfig);
    peerConnections.set(peerId, pc);

    if (localStream) {
        localStream.getTracks().forEach((track) => {
            pc.addTrack(track, localStream);
        });
    }

    pc.onicecandidate = (event) => {
        if (event.candidate) {
            sendSignal({
                type: "candidate",
                to: peerId,
                candidate: JSON.stringify(event.candidate),
            });
        }
    };

    pc.ontrack = (event) => {
        const [stream] = event.streams;
        if (!stream) {
            return;
        }

        let audioEl = remoteAudioEls.get(peerId);
        if (!audioEl) {
            audioEl = document.createElement("audio");
            audioEl.autoplay = true;
            audioEl.playsInline = true;
            remoteAudioEls.set(peerId, audioEl);
            document.body.appendChild(audioEl);
        }
        if (audioEl.srcObject !== stream) {
            audioEl.srcObject = stream;
        }
    };

    pc.onconnectionstatechange = () => {
        if (["failed", "closed", "disconnected"].includes(pc.connectionState)) {
            closePeer(peerId);
        }
    };

    return pc;
}

async function makeOffer(peerId) {
    const pc = createPeerConnection(peerId);
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    sendSignal({
        type: "offer",
        to: peerId,
        sdp: offer.sdp,
    });
}

async function handleOffer(peerId, sdp) {
    const pc = createPeerConnection(peerId);
    await pc.setRemoteDescription({ type: "offer", sdp });
    const answer = await pc.createAnswer();
    await pc.setLocalDescription(answer);
    sendSignal({
        type: "answer",
        to: peerId,
        sdp: answer.sdp,
    });
}

async function handleAnswer(peerId, sdp) {
    const pc = createPeerConnection(peerId);
    await pc.setRemoteDescription({ type: "answer", sdp });
}

async function handleCandidate(peerId, candidateRaw) {
    if (!candidateRaw) {
        return;
    }
    const pc = createPeerConnection(peerId);
    try {
        const candidate = JSON.parse(candidateRaw);
        await pc.addIceCandidate(candidate);
    } catch (err) {
        console.error("failed to add candidate", err);
    }
}

function closePeer(peerId) {
    const pc = peerConnections.get(peerId);
    if (pc) {
        pc.onicecandidate = null;
        pc.ontrack = null;
        pc.close();
        peerConnections.delete(peerId);
    }

    const audioEl = remoteAudioEls.get(peerId);
    if (audioEl) {
        audioEl.srcObject = null;
        audioEl.remove();
        remoteAudioEls.delete(peerId);
    }
}

function sendSignal(payload) {
    if (!voiceSocket || voiceSocket.readyState !== WebSocket.OPEN) {
        return;
    }
    voiceSocket.send(JSON.stringify(payload));
}

voiceJoinBtn.addEventListener("click", joinVoice);
voiceLeaveBtn.addEventListener("click", leaveVoice);
