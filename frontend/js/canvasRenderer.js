import { colorMap } from "./constants.js";

class CanvasRenderer {
  constructor(canvas) {
    document.getElementById("loading").remove();
    document.getElementById("captcha").style.display = "block";
    document.getElementById("canvas").style.display = "block";

    this.canvas = canvas;

    const canvasSize = Math.min(window.innerWidth - 50, 500);

    canvas.width = canvasSize;
    canvas.height = canvasSize;

    this.canvasSize = canvasSize;

    this.context = canvas.getContext("2d");
    this.gridSize = 100;
    this.pixelSize = canvasSize / this.gridSize;
    this.offsetX = 0;
    this.offsetY = 0;

    this.pixelUpdateTimestamps = [];

    this.pixelCountElement = document.getElementById("pixel-count");
    this.pixelCountElement.textContent = `You've updated ${
      localStorage.getItem("pixelCount") || 0
    } pixels`;
  }

  redraw(pixelData) {
    this.context.clearRect(0, 0, this.canvasSize, this.canvasSize);

    for (let i = 0; i < pixelData.length; i++) {
      const x = Math.floor(i % this.gridSize) * this.pixelSize - this.offsetX;
      const y = Math.floor(i / this.gridSize) * this.pixelSize - this.offsetY;
      const color = colorMap[pixelData[i]];

      this.context.fillStyle = color;
      this.context.fillRect(x, y, this.pixelSize, this.pixelSize);
    }
  }

  #applyOffsetLimits() {
    const maxOffset = this.gridSize * this.pixelSize - this.canvasSize;
    this.offsetX = Math.min(maxOffset, Math.max(0, this.offsetX));
    this.offsetY = Math.min(maxOffset, Math.max(0, this.offsetY));
  }

  zoom(pixelData, factor, cursorX, cursorY) {
    const oldPixelSize = this.pixelSize;
    this.pixelSize *= factor;
    const minPixelSize = this.canvasSize / 100;
    const maxPixelSize = this.canvasSize / 10;
    this.pixelSize = Math.max(
      minPixelSize,
      Math.min(maxPixelSize, this.pixelSize),
    );

    this.offsetX =
      (this.offsetX + cursorX) * (this.pixelSize / oldPixelSize) - cursorX;
    this.offsetY =
      (this.offsetY + cursorY) * (this.pixelSize / oldPixelSize) - cursorY;

    this.#applyOffsetLimits();

    this.redraw(pixelData);
  }

  #incrementPixelCount() {
    const oldPixelCount = localStorage.getItem("pixelCount") || 0;
    const newPixelCount = parseInt(oldPixelCount) + 1;
    localStorage.setItem("pixelCount", newPixelCount);
    this.pixelCountElement.textContent = `You've updated ${newPixelCount} pixels`;
  }

  updatePixel(pixelData, setPixelData, socket, selectedColor, x, y) {
    const pixelX = Math.floor((x - 4 + this.offsetX) / this.pixelSize);
    const pixelY = Math.floor((y - 4 + this.offsetY) / this.pixelSize);
    const index = pixelY * this.gridSize + pixelX;

    if (index < 0 || index >= 10000) {
      return;
    }

    if (pixelData[index] === selectedColor) {
      return;
    }

    if (this.#isRateLimited()) {
      return;
    }

    if (socket.readyState !== WebSocket.OPEN) {
      return;
    }

    this.#incrementPixelCount();

    const newPixelData =
      pixelData.substring(0, index) +
      selectedColor +
      pixelData.substring(index + 1);

    setPixelData(newPixelData);

    this.redraw(newPixelData);

    socket.send(
      JSON.stringify({
        type: "update",
        data: {
          index,
          color: selectedColor,
        },
      }),
    );
  }

  #isRateLimited() {
    const now = Date.now();

    if (
      this.pixelUpdateTimestamps.filter((timestamp) => now - timestamp < 5000)
        .length >= 20
    ) {
      return true;
    }

    this.pixelUpdateTimestamps = this.pixelUpdateTimestamps.filter(
      (timestamp) => now - timestamp < 5000,
    );

    this.pixelUpdateTimestamps.push(now);

    return false;
  }

  updateOffset(deltaX, deltaY) {
    this.offsetX -= deltaX;
    this.offsetY -= deltaY;

    this.#applyOffsetLimits();
  }
}

export default CanvasRenderer;
