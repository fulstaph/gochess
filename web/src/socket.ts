import { ClientMessage, ServerMessage } from "./types";

const TOKEN_KEY = "gochess_token";

export type MessageHandler = (msg: ServerMessage) => void;

export class ChessSocket {
  private ws: WebSocket | null = null;
  private onMessage: MessageHandler;
  private reconnectDelay = 500;
  private maxReconnectDelay = 8000;
  private closed = false;

  constructor(onMessage: MessageHandler) {
    this.onMessage = onMessage;
    this.connect();
  }

  private connect(): void {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const token = localStorage.getItem(TOKEN_KEY);
    const qs = token ? `?token=${encodeURIComponent(token)}` : "";
    const url = `${proto}//${location.host}/ws${qs}`;

    this.ws = new WebSocket(url);

    this.ws.onopen = () => {
      this.reconnectDelay = 500;
    };

    this.ws.onmessage = (event) => {
      const msg: ServerMessage = JSON.parse(event.data);
      if (msg.type === "session") {
        localStorage.setItem(TOKEN_KEY, msg.token);
      }
      this.onMessage(msg);
    };

    this.ws.onclose = () => {
      if (this.closed) return;
      setTimeout(() => this.connect(), this.reconnectDelay);
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  send(msg: ClientMessage): void {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  close(): void {
    this.closed = true;
    this.ws?.close();
  }
}
