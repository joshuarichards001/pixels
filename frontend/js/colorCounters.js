import { colorMap } from "./constants.js";

export const updateColorCounters = (pixelData) => {
  const colorCounts = new Map();
  const colorCounterContainer = document.querySelector(
    ".color-counter-container",
  );
  colorCounterContainer.textContent = "";

  for (const color of pixelData) {
    colorCounts.set(color, (colorCounts.get(color) || 0) + 1);
  }

  const sortedColorCounts = Array.from(colorCounts);
  sortedColorCounts.sort((a, b) => b[1] - a[1]);

  const fragment = document.createDocumentFragment();

  for (let i = 0; i < sortedColorCounts.length; i++) {
    const [color, count] = sortedColorCounts[i];
    const colorDiv = document.createElement("div");
    colorDiv.className = "color-counter";
    colorDiv.style.backgroundColor = colorMap[color];
    colorDiv.style.width = `${(count / 10000) * 100}%`;
    colorDiv.innerText = count;

    if (i < 3) {
      const star = document.createElement("span");
      star.innerText = "★";
      switch (i) {
        case 0:
          star.style.color = "#FFD700";
          break;
        case 1:
          star.style.color = "#C0C0C0";
          break;
        case 2:
          star.style.color = "#CD7F32";
          break;
      }
      colorDiv.appendChild(star);
    }

    fragment.appendChild(colorDiv);
  }

  colorCounterContainer.appendChild(fragment);
};
