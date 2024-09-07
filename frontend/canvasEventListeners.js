import ColorSelector from "./colorSelector.js";

export const initCanvasEventListeners = (
  canvas,
  getPixelData,
  setPixelData,
  canvasRenderer,
  getSocket,
) => {
  let isDragging = false;
  let isMouseDown = false;
  let lastX, lastY;
  let lastTouchDistance = 0;
  let touchStartTime = 0;
  const colorSelector = new ColorSelector();

  canvas.addEventListener("wheel", (e) => {
    e.preventDefault();
    const rect = canvas.getBoundingClientRect();
    const cursorX = e.clientX - rect.left - 4;
    const cursorY = e.clientY - rect.top - 4;
    const zoomFactor = e.deltaY > 0 ? 0.9 : 1.1;
    const pixelData = getPixelData();
    canvasRenderer.zoom(pixelData, zoomFactor, cursorX, cursorY);
  });

  canvas.addEventListener("mousedown", (e) => {
    lastX = e.clientX;
    lastY = e.clientY;
    isMouseDown = true;
  });

  canvas.addEventListener("mousemove", (e) => {
    if (
      isMouseDown &&
      (Math.abs(e.clientX - lastX) > 5 || Math.abs(e.clientY - lastY) > 5)
    ) {
      isDragging = true;
    }

    if (isDragging) {
      const deltaX = e.clientX - lastX;
      const deltaY = e.clientY - lastY;

      canvasRenderer.updateOffset(deltaX, deltaY);

      lastX = e.clientX;
      lastY = e.clientY;
      const pixelData = getPixelData();
      canvasRenderer.redraw(pixelData);
    }
  });

  canvas.addEventListener("mouseup", (e) => {
    if (!isDragging && touchStartTime === 0) {
      const socket = getSocket();
      const rect = canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      const pixelData = getPixelData();
      canvasRenderer.updatePixel(
        pixelData,
        setPixelData,
        socket,
        colorSelector.getSelectedColor(),
        x,
        y,
      );
    }

    isDragging = false;
    isMouseDown = false;
  });

  canvas.addEventListener("mouseleave", () => {
    isDragging = false;
    isMouseDown = false;
  });

  const getTouchDistance = (touches) => {
    const dx = touches[0].clientX - touches[1].clientX;
    const dy = touches[0].clientY - touches[1].clientY;
    return Math.sqrt(dx * dx + dy * dy);
  };

  canvas.addEventListener("touchstart", (e) => {
    touchStartTime = new Date().getTime();
    if (e.touches.length === 2) {
      lastTouchDistance = getTouchDistance(e.touches);
    } else if (e.touches.length === 1) {
      lastX = e.touches[0].clientX;
      lastY = e.touches[0].clientY;
      isMouseDown = true;
    }
  });

  canvas.addEventListener("touchmove", (e) => {
    e.preventDefault();
    const pixelData = getPixelData();

    if (e.touches.length === 2) {
      const currentDistance = getTouchDistance(e.touches);
      const zoomFactor = currentDistance / lastTouchDistance;

      const rect = canvas.getBoundingClientRect();
      const centerX =
        (e.touches[0].clientX + e.touches[1].clientX) / 2 - rect.left;
      const centerY =
        (e.touches[0].clientY + e.touches[1].clientY) / 2 - rect.top;

      canvasRenderer.zoom(pixelData, zoomFactor, centerX, centerY);

      lastTouchDistance = currentDistance;
    } else if (e.touches.length === 1 && isMouseDown) {
      const deltaX = e.touches[0].clientX - lastX;
      const deltaY = e.touches[0].clientY - lastY;

      canvasRenderer.updateOffset(deltaX, deltaY);

      lastX = e.touches[0].clientX;
      lastY = e.touches[0].clientY;
      canvasRenderer.redraw(pixelData);
    }
  });

  canvas.addEventListener("touchend", (e) => {
    const touchEndTime = new Date().getTime();
    const touchDuration = touchEndTime - touchStartTime;

    if (e.touches.length === 0 && touchDuration < 100) {
      const socket = getSocket();
      const rect = canvas.getBoundingClientRect();
      const x = lastX - rect.left;
      const y = lastY - rect.top;
      const pixelData = getPixelData();
      canvasRenderer.updatePixel(
        pixelData,
        setPixelData,
        socket,
        colorSelector.getSelectedColor(),
        x,
        y,
      );
    }

    isDragging = false;
    isMouseDown = false;
  });

  document.getElementById("zoom-in").addEventListener("click", () => {
    const pixelData = getPixelData();
    canvasRenderer.zoom(pixelData, 1.2, canvas.width / 2, canvas.height / 2);
  });

  document.getElementById("zoom-out").addEventListener("click", () => {
    const pixelData = getPixelData();
    canvasRenderer.zoom(pixelData, 0.8, canvas.width / 2, canvas.height / 2);
  });
};
