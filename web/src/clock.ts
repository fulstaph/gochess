// GameClock renders a countdown for one player and keeps it in sync with
// server snapshots. The server is the sole authority — the client only
// interpolates between updates.
export class GameClock {
  private el: HTMLElement;
  private remaining: number = 0; // milliseconds
  private active = false;
  private intervalId: ReturnType<typeof setInterval> | null = null;
  private lastTickAt = 0;

  constructor(el: HTMLElement) {
    this.el = el;
  }

  // sync updates the remaining time from a server snapshot and whether
  // this side's clock is currently running.
  sync(remainingMs: number, running: boolean): void {
    this.remaining = remainingMs;
    this.lastTickAt = Date.now();

    if (running && !this.active) {
      this.active = true;
      this.intervalId = setInterval(() => this.tick(), 100);
    } else if (!running) {
      // Stop (or keep stopped) without calling stop() which would clear active
      // before we set it — just cancel the interval directly.
      if (this.intervalId !== null) {
        clearInterval(this.intervalId);
        this.intervalId = null;
      }
      this.active = false;
    }
    // When already running, we only updated remaining+lastTickAt above,
    // which re-anchors the local countdown to the server snapshot.

    this.render();
  }

  stop(): void {
    this.active = false;
    if (this.intervalId !== null) {
      clearInterval(this.intervalId);
      this.intervalId = null;
    }
  }

  private tick(): void {
    const now = Date.now();
    const elapsed = now - this.lastTickAt;
    this.lastTickAt = now;
    this.remaining = Math.max(0, this.remaining - elapsed);
    this.render();

    if (this.remaining === 0) {
      this.stop();
    }
  }

  private render(): void {
    const totalSec = Math.ceil(this.remaining / 1000);
    const min = Math.floor(totalSec / 60);
    const sec = totalSec % 60;
    this.el.textContent = `${min}:${sec.toString().padStart(2, "0")}`;

    // Low-time warning
    if (this.remaining < 10_000) {
      this.el.classList.add("clock-low");
    } else {
      this.el.classList.remove("clock-low");
    }
  }
}
