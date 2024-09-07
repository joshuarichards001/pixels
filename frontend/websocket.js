import { updateColorCounters } from "./colorCounters.js";
import { websocketUrl } from "./constants.js";

export const handleWebsocket = (
  setSocket,
  hCaptchaToken,
  getPixelData,
  setPixelData,
  canvas,
  canvasRenderer,
) => {
  const socket = new WebSocket(websocketUrl, hCaptchaToken);
  const loading = document.getElementById("loading");

  socket.onopen = () => {
    console.log("WebSocket connection established");
  };

  socket.onmessage = (event) => {
    if (event.data === "rate limit exceeded") {
      return;
    }

    if (event.data === "client limit exceeded") {
      loading.textContent = "Client limit exceeded. Please try again later.";
      return;
    }

    const messageData = JSON.parse(event.data);

    let newPixelData = "";

    if (messageData.type === "initial") {
      newPixelData = messageData.data;
      loading.style.display = "none";
      canvas.style.display = "block";
    } else if (messageData.type === "update") {
      const pixelData = getPixelData();
      const { index, color } = messageData.data;
      newPixelData =
        pixelData.substring(0, index) + color + pixelData.substring(index + 1);
    }

    document.getElementById("client-count").textContent =
      messageData.clientCount;
    canvasRenderer.redraw(newPixelData);
    setPixelData(newPixelData);
    updateColorCounters(newPixelData);
  };

  socket.onerror = (error) => {
    console.error("WebSocket error:", error);
    loading.textContent = "Error connecting to server. Please try again later.";
  };

  socket.onclose = () => {
    console.log("WebSocket connection closed");
  };

  setSocket(socket);
};
