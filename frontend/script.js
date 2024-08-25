const colorMap = {
  "0": "#FFFFFF", // white
  "1": "#00FF00", // green
  "2": "#FFFF00", // yellow
  "3": "#FF0000", // red
  "4": "#FFA500", // orange
  "5": "#800080", // purple
  "6": "#0000FF", // blue
  "7": "#008080", // teal
  "8": "#FFC0CB", // pink
  "9": "#000000", // black
};

window.onload = () => {
  const canvas = document.getElementById("canvas");
  const context = canvas.getContext("2d");
  const colorButtons = document.querySelectorAll(".color-button");
  
  const gridSize = 100;
  let pixelData = '';
  let pixelSize = 3;
  let offsetX = 0;
  let offsetY = 0;
  let isDragging = false;
  let isMouseDown = false;
  let lastX, lastY;
  let selectedColor = "9";

  const socket = new WebSocket('ws://localhost:8080/ws');

  socket.onopen = () => {
    console.log('WebSocket connection established');
  };

  socket.onmessage = (event) => {
    pixelData = JSON.parse(event.data).data;
    redraw();
  };

  socket.onerror = (error) => {
    console.error('WebSocket error:', error);
  };

  socket.onclose = () => {
    console.log('WebSocket connection closed');
  };

  const redraw = () => {
    canvas.width = 300;
    canvas.height = 300;
    
    context.clearRect(0, 0, canvas.width, canvas.height);
    
    for (let i = 0; i < pixelData.length; i++) {
      const x = ((i % gridSize) * pixelSize) - offsetX;
      const y = (Math.floor(i / gridSize) * pixelSize) - offsetY;
      const color = colorMap[pixelData[i]];
      
      context.fillStyle = color;
      context.fillRect(x, y, pixelSize, pixelSize);
    }
  }
  
  const applyOffsetLimits = () => {
    const maxOffset = (gridSize * pixelSize) - canvas.width;
    offsetX = Math.min(maxOffset, Math.max(0, offsetX));
    offsetY = Math.min(maxOffset, Math.max(0, offsetY));
  }

  const zoom = (factor, cursorX, cursorY) => {
    const oldPixelSize = pixelSize;
    pixelSize *= factor;
    pixelSize = Math.max(3, Math.min(30, pixelSize));

    offsetX = (offsetX + cursorX) * (pixelSize / oldPixelSize) - cursorX;
    offsetY = (offsetY + cursorY) * (pixelSize / oldPixelSize) - cursorY;
    
    applyOffsetLimits();

    redraw();
  }

  const updatePixel = (x, y) => {
    const pixelX = Math.floor((x + offsetX) / pixelSize);
    const pixelY = Math.floor((y + offsetY) / pixelSize);
    const index = pixelY * gridSize + pixelX;
    
    if (index >= 0 && index < pixelData.length) {
      socket.send(JSON.stringify({
        type: 'update',
        data: {
          index,
          color: selectedColor,
        },
      }));
    }
  }

  canvas.addEventListener('wheel', (e) => {
    e.preventDefault();
    const rect = canvas.getBoundingClientRect();
    const cursorX = e.clientX - rect.left;
    const cursorY = e.clientY - rect.top;
    const zoomFactor = e.deltaY > 0 ? 0.9 : 1.1;
    zoom(zoomFactor, cursorX, cursorY);
  });

  canvas.addEventListener('mousedown', (e) => {
    lastX = e.clientX;
    lastY = e.clientY;
    isMouseDown = true;
  });

  canvas.addEventListener('mousemove', (e) => {
    if (isMouseDown && (Math.abs(e.clientX - lastX) > 5 || Math.abs(e.clientY - lastY) > 5)) {
      isDragging = true;
    }
    
    if (isDragging) {
      const deltaX = e.clientX - lastX;
      const deltaY = e.clientY - lastY;

      offsetX -= deltaX;
      offsetY -= deltaY;
      
      applyOffsetLimits();

      lastX = e.clientX;
      lastY = e.clientY;
      redraw();
    }
  });

  canvas.addEventListener('mouseup', (e) => {
    if (!isDragging) {
      const rect = canvas.getBoundingClientRect();
      const x = e.clientX - rect.left;
      const y = e.clientY - rect.top;
      updatePixel(x, y);
    }
    
    isDragging = false;
    isMouseDown = false;
  });

  canvas.addEventListener('mouseleave', () => {
    isDragging = false;
    isMouseDown = false;
  });

  colorButtons.forEach(button => {
    button.addEventListener('click', () => {
      selectedColor = Object.keys(colorMap).find(key => colorMap[key] === button.dataset.color);
    });
  });
}