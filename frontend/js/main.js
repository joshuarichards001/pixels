import { initCanvasEventListeners } from "./canvasEventListeners.js";
import CanvasRenderer from "./canvasRenderer.js";
import { updateColorCounters } from "./colorCounters.js";
import { pixelsUrl } from "./constants.js";
import { handleWebsocket } from "./websocket.js";

window.onload = async () => {
  const canvas = document.getElementById("canvas");

  let socket;
  const getSocket = () => socket;
  const setSocket = (newSocket) => {
    socket = newSocket;
  };

  const response = await fetch(pixelsUrl);
  let pixelData = await response.text();

  const getPixelData = () => pixelData;
  const setPixelData = (newPixelData) => {
    pixelData = newPixelData;
  };

  const canvasRenderer = new CanvasRenderer(canvas);
  canvasRenderer.redraw(pixelData);
  updateColorCounters(pixelData);

  initCanvasEventListeners(
    canvas,
    getPixelData,
    setPixelData,
    canvasRenderer,
    getSocket,
  );

  window.connectToWebsocket = function (hCaptchaToken) {
    handleWebsocket(
      setSocket,
      hCaptchaToken,
      getPixelData,
      setPixelData,
      canvas,
      canvasRenderer,
    );
  };
};
