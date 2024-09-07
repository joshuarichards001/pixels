import { initCanvasEventListeners } from "./canvasEventListeners.js";
import CanvasRenderer from "./canvasRenderer.js";
import { handleWebsocket } from "./websocket.js";

window.onload = () => {
  const canvas = document.getElementById("canvas");

  let socket;
  const getSocket = () => socket;
  const setSocket = (newSocket) => {
    socket = newSocket;
  };

  let pixelData = "";
  const getPixelData = () => pixelData;
  const setPixelData = (newPixelData) => {
    pixelData = newPixelData;
  };

  const canvasRenderer = new CanvasRenderer(canvas);

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
