import { RoomInfo } from "./types";

export type CreateRoomOpts = {
  aiMode: string;
  aiDepth: number;
  color: string;
  timeControl: string;
};

export type LobbyCallbacks = {
  onCreateRoom: (opts: CreateRoomOpts) => void;
  onJoinRoom: (roomId: string) => void;
  onRefresh: () => void;
};

export class Lobby {
  private el: HTMLElement;
  private roomListEl: HTMLElement;
  private displayNameEl: HTMLElement;
  private callbacks: LobbyCallbacks;

  // Dialog elements
  private createDialog: HTMLElement;
  private modeSelect: HTMLSelectElement;
  private colorSelect: HTMLSelectElement;
  private depthSelect: HTMLSelectElement;
  private tcSelect: HTMLSelectElement;
  private depthRow: HTMLElement;

  constructor(el: HTMLElement, callbacks: LobbyCallbacks) {
    this.el = el;
    this.callbacks = callbacks;

    this.roomListEl = el.querySelector("#lobby-room-list")!;
    // display name lives in the site header, outside the lobby element
    this.displayNameEl = document.getElementById("lobby-display-name")!;
    // dialog lives outside the lobby element at the app root
    this.createDialog = document.getElementById("create-room-dialog")!;
    this.modeSelect = this.createDialog.querySelector("#create-mode-select") as HTMLSelectElement;
    this.colorSelect = this.createDialog.querySelector("#create-color-select") as HTMLSelectElement;
    this.depthSelect = this.createDialog.querySelector("#create-depth-select") as HTMLSelectElement;
    this.tcSelect = this.createDialog.querySelector("#create-tc-select") as HTMLSelectElement;
    this.depthRow = this.createDialog.querySelector("#create-depth-row")!;

    el.querySelector("#lobby-create-btn")!.addEventListener("click", () => {
      this.createDialog.classList.remove("hidden");
    });

    el.querySelector("#lobby-refresh-btn")!.addEventListener("click", () => {
      this.callbacks.onRefresh();
    });

    this.createDialog.querySelector("#create-dialog-cancel")!.addEventListener("click", () => {
      this.createDialog.classList.add("hidden");
    });

    this.createDialog.querySelector("#create-dialog-start")!.addEventListener("click", () => {
      this.createDialog.classList.add("hidden");
      this.callbacks.onCreateRoom({
        aiMode: this.modeSelect.value,
        aiDepth: parseInt(this.depthSelect.value, 10),
        color: this.colorSelect.value,
        timeControl: this.tcSelect.value,
      });
    });

    this.modeSelect.addEventListener("change", () => {
      const vsAI = this.modeSelect.value !== "none" && this.modeSelect.value !== "";
      this.depthRow.classList.toggle("hidden", !vsAI);
    });
  }

  setDisplayName(name: string): void {
    this.displayNameEl.textContent = name;
  }

  renderRooms(rooms: RoomInfo[]): void {
    this.roomListEl.innerHTML = "";

    const waiting = rooms.filter((r) => r.status === "waiting");
    const playing = rooms.filter((r) => r.status === "playing");

    if (rooms.length === 0) {
      const empty = document.createElement("div");
      empty.className = "lobby-empty";
      empty.textContent = "No open rooms. Create one!";
      this.roomListEl.appendChild(empty);
      return;
    }

    const renderGroup = (title: string, list: RoomInfo[]) => {
      if (list.length === 0) return;
      const heading = document.createElement("div");
      heading.className = "room-group-heading";
      heading.textContent = title;
      this.roomListEl.appendChild(heading);

      for (const room of list) {
        this.roomListEl.appendChild(this.buildRoomRow(room));
      }
    };

    renderGroup("Open — waiting for opponent", waiting);
    renderGroup("In progress", playing);
  }

  private buildRoomRow(room: RoomInfo): HTMLElement {
    const row = document.createElement("div");
    row.className = "room-row";

    const whiteStr = room.whiteRating ? `${room.whiteName} (${room.whiteRating})` : room.whiteName;
    const blackStr = room.blackRating ? `${room.blackName} (${room.blackRating})` : room.blackName;
    const tc = room.timeControl ? ` · ${room.timeControl}` : "";
    const specs = room.spectators ? ` · ${room.spectators} watching` : "";

    const info = document.createElement("div");
    info.className = "room-info";

    const roomIdSpan = document.createElement("span");
    roomIdSpan.className = "room-id";
    roomIdSpan.textContent = room.roomId + tc + specs;

    const playersSpan = document.createElement("span");
    playersSpan.className = "room-players";
    playersSpan.textContent = `${whiteStr} vs ${blackStr}`;

    info.appendChild(roomIdSpan);
    info.appendChild(playersSpan);

    row.appendChild(info);

    if (room.status === "waiting") {
      const joinBtn = document.createElement("button");
      joinBtn.className = "btn-join";
      joinBtn.textContent = "Join";
      joinBtn.addEventListener("click", () => this.callbacks.onJoinRoom(room.roomId));
      row.appendChild(joinBtn);
    } else {
      const badge = document.createElement("span");
      badge.className = "room-badge room-badge-playing";
      badge.textContent = "Playing";
      row.appendChild(badge);
    }

    return row;
  }

  show(): void {
    this.el.classList.remove("hidden");
  }

  hide(): void {
    this.el.classList.add("hidden");
  }
}
