export interface GameRecord {
  id: string;
  whiteId: string;
  blackId: string;
  pgn: string;
  result: string;
  timeControl: string;
  rated: boolean;
  startedAt: string;
  finishedAt: string;
}

export async function fetchPlayerGames(playerID: string, limit = 20, offset = 0): Promise<GameRecord[]> {
  const url = `/api/games?player=${encodeURIComponent(playerID)}&limit=${limit}&offset=${offset}`;
  const resp = await fetch(url);
  if (!resp.ok) return [];
  return resp.json();
}

export function downloadPGN(game: GameRecord): void {
  const blob = new Blob([game.pgn], { type: "text/plain" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `gochess-${game.id}.pgn`;
  a.click();
  URL.revokeObjectURL(a.href);
}

export class GameHistoryPanel {
  private el: HTMLElement;
  private playerID = "";

  constructor(el: HTMLElement) {
    this.el = el;
  }

  async load(playerID: string): Promise<void> {
    this.playerID = playerID;
    this.el.innerHTML = "";

    const games = await fetchPlayerGames(playerID);
    if (games.length === 0) {
      const empty = document.createElement("p");
      empty.className = "history-empty";
      empty.textContent = "No games played yet.";
      this.el.appendChild(empty);
      return;
    }

    for (const g of games) {
      this.el.appendChild(this.buildRow(g));
    }
  }

  private buildRow(g: GameRecord): HTMLElement {
    const row = document.createElement("div");
    row.className = "history-row";

    const info = document.createElement("div");
    info.className = "history-info";

    const result = document.createElement("span");
    result.className = "history-result";
    result.textContent = g.result || "—";

    const meta = document.createElement("span");
    meta.className = "history-meta";
    const date = new Date(g.finishedAt).toLocaleDateString();
    meta.textContent = `${date}${g.timeControl ? " · " + g.timeControl : ""}${g.rated ? " · rated" : ""}`;

    info.appendChild(result);
    info.appendChild(meta);
    row.appendChild(info);

    if (g.pgn) {
      const dl = document.createElement("button");
      dl.className = "btn-dl-pgn";
      dl.textContent = "PGN";
      dl.title = "Download PGN";
      dl.addEventListener("click", () => downloadPGN(g));
      row.appendChild(dl);
    }

    return row;
  }
}
