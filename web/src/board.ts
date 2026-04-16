import { GameState, LegalMove } from "./types";

const PIECE_MAP: Record<string, string> = {
  K: "\u2654", Q: "\u2655", R: "\u2656", B: "\u2657", N: "\u2658", P: "\u2659",
  k: "\u265A", q: "\u265B", r: "\u265C", b: "\u265D", n: "\u265E", p: "\u265F",
};

const FILES = "abcdefgh";
const DRAG_THRESHOLD = 5; // pixels before a pointerdown becomes a drag

type DragState = {
  from: string;
  ghost: HTMLElement;
  startX: number;
  startY: number;
  dragging: boolean;
};

export type MoveCallback = (from: string, to: string, promotion?: string) => void;

export class Board {
  private el: HTMLElement;
  private flipped = false;
  private selectedSquare: string | null = null;
  private state: GameState | null = null;
  private onMove: MoveCallback;
  private pendingPromotion: { from: string; to: string } | null = null;
  private lastAnimatedMove: string | null = null;

  // Drag state
  private drag: DragState | null = null;

  // Pre-move: queued while it's not the player's turn
  private preMoveFrom: string | null = null;
  private preMoveTo: string | null = null;

  constructor(el: HTMLElement, onMove: MoveCallback) {
    this.el = el;
    this.onMove = onMove;

    // Delegate square interactions to the board container — avoids 128 listeners per render.
    this.el.addEventListener("pointerdown", (e) => {
      const sq = squareAt(e.target as Element);
      if (sq) this.onPointerDown(e, sq);
    });
    this.el.addEventListener("click", (e) => {
      const sq = squareAt(e.target as Element);
      if (sq) this.handleClick(sq);
    });

    window.addEventListener("pointermove", (e) => this.onPointerMove(e));
    window.addEventListener("pointerup", (e) => this.onPointerUp(e));
  }

  flip(): void {
    this.flipped = !this.flipped;
    if (this.state) this.render(this.state);
  }

  setFlipped(flipped: boolean): void {
    this.flipped = flipped;
  }

  render(state: GameState): void {
    const prevState = this.state;
    this.state = state;

    // If a pre-move is queued and it's now our turn, fire it.
    if (
      this.preMoveFrom &&
      this.preMoveTo &&
      prevState &&
      state.legalMoves.length > 0
    ) {
      const from = this.preMoveFrom;
      const to = this.preMoveTo;
      this.preMoveFrom = null;
      this.preMoveTo = null;
      const moves = state.legalMoves.filter((m) => m.from === from && m.to === to);
      if (moves.length > 0) {
        const promos = moves.filter((m) => m.promotion);
        if (promos.length > 0) {
          this.pendingPromotion = { from, to };
          this.el.innerHTML = "";
          this.drawSquares(state);
          this.showPromotionDialog(promos);
          return;
        }
        this.onMove(from, to);
        return;
      }
    }

    this.el.innerHTML = "";
    this.drawSquares(state);

    if (state.lastMove) {
      const moveKey = `${state.lastMove.from}-${state.lastMove.to}`;
      if (moveKey !== this.lastAnimatedMove) {
        this.lastAnimatedMove = moveKey;
        this.animateMove(state.lastMove.from, state.lastMove.to);
      }
    }
  }

  private drawSquares(state: GameState): void {
    const legalFromSelected = this.selectedSquare
      ? state.legalMoves.filter((m) => m.from === this.selectedSquare)
      : [];
    const legalTargets = new Set(legalFromSelected.map((m) => m.to));

    const captureTargets = new Set<string>();
    for (const m of legalFromSelected) {
      const [tr, tc] = squareToCoords(m.to);
      if (state.board[tr][tc] !== "") captureTargets.add(m.to);
    }

    let kingSquare: string | null = null;
    if (state.isCheck) {
      const kingPiece = state.turn === "white" ? "K" : "k";
      outer: for (let r = 0; r < 8; r++) {
        for (let c = 0; c < 8; c++) {
          if (state.board[r][c] === kingPiece) {
            kingSquare = coordsToSquare(r, c);
            break outer;
          }
        }
      }
    }

    for (let row = 0; row < 8; row++) {
      for (let col = 0; col < 8; col++) {
        const r = this.flipped ? 7 - row : row;
        const c = this.flipped ? 7 - col : col;
        const sq = coordsToSquare(r, c);
        const isLight = (r + c) % 2 === 0;

        const div = document.createElement("div");
        div.className = "square " + (isLight ? "light" : "dark");
        div.dataset.square = sq;

        if (state.lastMove && (sq === state.lastMove.from || sq === state.lastMove.to)) {
          div.classList.add("highlight");
        }
        if (sq === this.selectedSquare) div.classList.add("selected");
        if (legalTargets.has(sq)) {
          div.classList.add(captureTargets.has(sq) ? "legal-capture" : "legal-target");
        }
        if (sq === kingSquare) div.classList.add("check");
        if (sq === this.preMoveFrom || sq === this.preMoveTo) {
          div.classList.add("premove");
        }

        const piece = state.board[r][c];
        if (piece && PIECE_MAP[piece]) {
          const span = document.createElement("span");
          span.className = piece >= "a" ? "piece piece-black" : "piece piece-white";
          span.textContent = PIECE_MAP[piece];
          div.appendChild(span);
        }

        if (row === 7) {
          const fileLabel = document.createElement("span");
          fileLabel.className = "file-label";
          fileLabel.textContent = FILES[c];
          div.appendChild(fileLabel);
        }
        if (col === 0) {
          const rankLabel = document.createElement("span");
          rankLabel.className = "rank-label";
          rankLabel.textContent = String(8 - r);
          div.appendChild(rankLabel);
        }

        this.el.appendChild(div);
      }
    }
  }

  // ---- Drag and drop ----

  private onPointerDown(e: PointerEvent, sq: string): void {
    if (!this.state || this.state.isGameOver) return;
    const [r, c] = squareToCoords(sq);
    const piece = this.state.board[r][c];
    if (!piece || !PIECE_MAP[piece]) return;

    // Only allow dragging the side-to-move's pieces (or pre-move for own pieces).
    const isOwnTurn = this.isPieceOfCurrentTurn(piece);
    const isOwnPiece = this.isOwnPiece(piece);
    if (!isOwnTurn && !isOwnPiece) return;

    const squareEl = this.el.querySelector(`[data-square="${sq}"]`) as HTMLElement | null;
    if (!squareEl) return;

    // Create ghost element for dragging.
    const ghost = document.createElement("div");
    ghost.className = "drag-ghost";
    ghost.textContent = PIECE_MAP[piece];
    ghost.style.left = e.clientX - 24 + "px";
    ghost.style.top = e.clientY - 24 + "px";
    document.body.appendChild(ghost);

    this.drag = { from: sq, ghost, startX: e.clientX, startY: e.clientY, dragging: false };
    e.preventDefault();
  }

  private onPointerMove(e: PointerEvent): void {
    if (!this.drag) return;
    const dx = e.clientX - this.drag.startX;
    const dy = e.clientY - this.drag.startY;

    if (!this.drag.dragging && Math.sqrt(dx * dx + dy * dy) >= DRAG_THRESHOLD) {
      this.drag.dragging = true;
      this.drag.ghost.style.display = "block";
      // Hide the piece on the source square during drag.
      const fromEl = this.el.querySelector(`[data-square="${this.drag.from}"]`);
      fromEl?.querySelector(".piece")?.classList.add("piece-dragging");
    }

    if (this.drag.dragging) {
      this.drag.ghost.style.left = e.clientX - 24 + "px";
      this.drag.ghost.style.top = e.clientY - 24 + "px";
    }
  }

  private onPointerUp(e: PointerEvent): void {
    if (!this.drag) return;
    const { from, ghost, dragging } = this.drag;
    this.drag = null;

    ghost.remove();

    // Restore piece visibility on source square.
    const fromEl = this.el.querySelector(`[data-square="${from}"]`);
    fromEl?.querySelector(".piece")?.classList.remove("piece-dragging");

    if (!dragging) return; // Not a drag — let click handler run.

    const target = document.elementFromPoint(e.clientX, e.clientY);
    const squareEl = target?.closest("[data-square]") as HTMLElement | null;
    if (!squareEl) return;

    const to = squareEl.dataset.square!;
    if (to === from) return;

    this.tryMove(from, to);
  }

  // ---- Click handling ----

  private handleClick(sq: string): void {
    if (!this.state || this.state.isGameOver) return;
    if (this.drag?.dragging) return; // Drag in progress — ignore click.

    if (this.selectedSquare && this.selectedSquare !== sq) {
      const moves = this.state.legalMoves.filter(
        (m) => m.from === this.selectedSquare && m.to === sq,
      );
      if (moves.length > 0) {
        const from = this.selectedSquare;
        this.selectedSquare = null;
        const promos = moves.filter((m) => m.promotion);
        if (promos.length > 0) {
          this.pendingPromotion = { from, to: sq };
          this.showPromotionDialog(promos);
          return;
        }
        this.onMove(from, sq);
        return;
      }
    }

    const [r, c] = squareToCoords(sq);
    const piece = this.state.board[r][c];

    // Pre-move: click a piece when it's not your turn.
    if (!this.isPieceOfCurrentTurn(piece) && this.isOwnPiece(piece)) {
      if (this.selectedSquare) {
        // Second click — set pre-move destination.
        this.preMoveTo = sq;
        this.preMoveFrom = this.selectedSquare;
        this.selectedSquare = null;
        if (this.state) this.render(this.state);
        return;
      }
      this.selectedSquare = sq;
      if (this.state) this.render(this.state);
      return;
    }

    if (piece && this.isPieceOfCurrentTurn(piece)) {
      const hasLegalMoves = this.state.legalMoves.some((m) => m.from === sq);
      this.selectedSquare = hasLegalMoves ? sq : null;
    } else {
      this.selectedSquare = null;
    }
    if (this.state) this.render(this.state);
  }

  private tryMove(from: string, to: string): void {
    if (!this.state) return;

    const isOwnTurn = this.state.legalMoves.some((m) => m.from === from);

    if (!isOwnTurn) {
      // Queue as pre-move.
      this.preMoveFrom = from;
      this.preMoveTo = to;
      if (this.state) this.render(this.state);
      return;
    }

    const moves = this.state.legalMoves.filter((m) => m.from === from && m.to === to);
    if (moves.length === 0) return;

    const promos = moves.filter((m) => m.promotion);
    if (promos.length > 0) {
      this.pendingPromotion = { from, to };
      this.showPromotionDialog(promos);
      return;
    }
    this.onMove(from, to);
  }

  private isPieceOfCurrentTurn(piece: string): boolean {
    if (!this.state) return false;
    if (this.state.turn === "white") return piece >= "A" && piece <= "Z";
    return piece >= "a" && piece <= "z";
  }

  // Returns true if this piece belongs to the local player (based on playerColor).
  private isOwnPiece(piece: string): boolean {
    if (!this.state) return false;
    const color = this.state.playerColor;
    if (!color || color === "spectator") return false;
    if (color === "white") return piece >= "A" && piece <= "Z";
    return piece >= "a" && piece <= "z";
  }

  private animateMove(from: string, to: string): void {
    const fromEl = this.el.querySelector(`[data-square="${from}"]`) as HTMLElement | null;
    const toEl = this.el.querySelector(`[data-square="${to}"]`) as HTMLElement | null;
    if (!fromEl || !toEl) return;

    const pieceEl = toEl.querySelector(".piece") as HTMLElement | null;
    if (!pieceEl) return;

    const fromRect = fromEl.getBoundingClientRect();
    const toRect = toEl.getBoundingClientRect();
    const dx = fromRect.left - toRect.left;
    const dy = fromRect.top - toRect.top;

    pieceEl.style.transition = "none";
    pieceEl.style.transform = `translate(${dx}px, ${dy}px)`;

    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        pieceEl.style.transition = "transform 0.18s ease";
        pieceEl.style.transform = "";
      });
    });
  }

  private showPromotionDialog(moves: LegalMove[]): void {
    const dialog = document.getElementById("promotion-dialog")!;
    const piecesContainer = document.getElementById("promotion-pieces")!;
    piecesContainer.innerHTML = "";

    const isWhite = this.state!.turn === "white";
    const promoMap: Record<string, string> = isWhite
      ? { q: "\u2655", r: "\u2656", b: "\u2657", n: "\u2658" }
      : { q: "\u265B", r: "\u265C", b: "\u265D", n: "\u265E" };

    for (const mv of moves) {
      const btn = document.createElement("button");
      btn.textContent = promoMap[mv.promotion!] || mv.promotion!;
      btn.addEventListener("click", () => {
        dialog.classList.add("hidden");
        this.selectedSquare = null;
        this.onMove(mv.from, mv.to, mv.promotion);
        this.pendingPromotion = null;
      });
      piecesContainer.appendChild(btn);
    }

    dialog.classList.remove("hidden");
    dialog.addEventListener(
      "click",
      (e) => {
        if (e.target === dialog) {
          dialog.classList.add("hidden");
          this.pendingPromotion = null;
          this.selectedSquare = null;
          if (this.state) this.render(this.state);
        }
      },
      { once: true },
    );
  }

  clearSelection(): void {
    this.selectedSquare = null;
    this.preMoveFrom = null;
    this.preMoveTo = null;
  }
}

function squareAt(el: Element): string | null {
  return (el.closest("[data-square]") as HTMLElement | null)?.dataset.square ?? null;
}

function squareToCoords(sq: string): [number, number] {
  const c = sq.charCodeAt(0) - 97; // 'a' = 97
  const r = 8 - parseInt(sq[1], 10);
  return [r, c];
}

function coordsToSquare(r: number, c: number): string {
  return FILES[c] + String(8 - r);
}
