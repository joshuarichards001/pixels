export const colorMap = {
  0: "#FFFFFF", // white
  1: "#74B63E", // green
  2: "#FFCE33", // yellow
  3: "#CC421D", // red
  4: "#FF8533", // orange
  5: "#87308C", // purple
  6: "#1D70A2", // blue
  7: "#079D9D", // teal
  8: "#F05689", // pink
  9: "#000000", // black
};

const production = true;

export const websocketUrl = production
  ? "wss://pixels-backend.fly.dev/ws"
  : "ws://localhost:8080/ws";

export const pixelsUrl = production
  ? "https://pixels-backend.fly.dev/pixels"
  : "http://localhost:8080/pixels";
