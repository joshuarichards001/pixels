import { colorMap } from "./constants.js";

class ColorSelector {
  constructor() {
    this.selectedColor = localStorage.getItem("selectedColor") || "9";
    this.setupColorButtons();
  }

  setupColorButtons() {
    let selectedButton = document.querySelector(
      `.color-button[data-color="${colorMap[this.selectedColor]}"]`,
    );
    selectedButton.classList.add("selected");
    
    const colorButtons = document.querySelectorAll(".color-button");
    colorButtons.forEach((button) => {
      button.addEventListener("click", () => {
        this.selectedColor = Object.keys(colorMap).find(
          (key) => colorMap[key] === button.dataset.color,
        );

        localStorage.setItem("selectedColor", this.selectedColor);

        selectedButton.classList.remove("selected");
        button.classList.add("selected");
        selectedButton = button;
      });
    });
  }

  getSelectedColor() {
    return this.selectedColor;
  }
}

export default ColorSelector;
