// Sound effects synthesized via Web Audio API — no external files needed.

export class SoundPlayer {
  private ctx: AudioContext | null = null;
  enabled = true;

  private getCtx(): AudioContext {
    if (!this.ctx) this.ctx = new AudioContext();
    return this.ctx;
  }

  playMove(): void    { this.play(() => this.tone(800, 0.06, "square",   0.08)); }
  playCapture(): void { this.play(() => this.tone(500, 0.1,  "sawtooth", 0.12)); }
  playCastle(): void  {
    this.play(() => {
      this.tone(700, 0.07, "square", 0.08);
      setTimeout(() => this.tone(900, 0.07, "square", 0.08), 100);
    });
  }
  playCheck(): void {
    this.play(() => {
      this.tone(900, 0.15, "sine", 0.18);
      setTimeout(() => this.tone(750, 0.12, "sine", 0.15), 120);
    });
  }
  playGameEnd(): void {
    this.play(() => {
      this.tone(523, 0.2, "sine", 0.25);
      setTimeout(() => this.tone(659, 0.2, "sine", 0.25), 150);
      setTimeout(() => this.tone(784, 0.3, "sine", 0.35), 300);
    });
  }

  private play(fn: () => void): void {
    if (this.enabled) fn();
  }

  // freq: Hz, duration: seconds, type: oscillator waveform, gain: volume 0-1
  private tone(freq: number, duration: number, type: OscillatorType, gain: number): void {
    try {
      const ctx = this.getCtx();
      const osc = ctx.createOscillator();
      const gainNode = ctx.createGain();

      osc.type = type;
      osc.frequency.setValueAtTime(freq, ctx.currentTime);
      gainNode.gain.setValueAtTime(gain, ctx.currentTime);
      gainNode.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + duration);

      osc.connect(gainNode);
      gainNode.connect(ctx.destination);
      osc.start(ctx.currentTime);
      osc.stop(ctx.currentTime + duration);
    } catch {
      // AudioContext may be blocked until user gesture — silently ignore.
    }
  }
}
