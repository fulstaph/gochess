const STORAGE_KEY = "gochess_settings";

export type BoardTheme = "green" | "brown" | "blue" | "grey";

export interface SettingsData {
  sound: boolean;
  theme: BoardTheme;
  autoFlip: boolean;
}

const DEFAULTS: SettingsData = {
  sound: true,
  theme: "green",
  autoFlip: false,
};

export class Settings {
  private data: SettingsData;
  private onChange: (data: SettingsData) => void;

  constructor(onChange: (data: SettingsData) => void) {
    this.onChange = onChange;
    this.data = { ...DEFAULTS };
    this.load();
    // Sync consumers with the loaded values immediately.
    this.onChange(this.data);
  }

  get sound(): boolean { return this.data.sound; }
  get theme(): BoardTheme { return this.data.theme; }
  get autoFlip(): boolean { return this.data.autoFlip; }

  set sound(v: boolean) { this.update({ sound: v }); }
  set theme(v: BoardTheme) { this.update({ theme: v }); }
  set autoFlip(v: boolean) { this.update({ autoFlip: v }); }

  private update(patch: Partial<SettingsData>): void {
    this.data = { ...this.data, ...patch };
    this.save();
    this.onChange(this.data);
  }

  private load(): void {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      if (raw) {
        const parsed = JSON.parse(raw) as Partial<SettingsData>;
        this.data = { ...DEFAULTS, ...parsed };
      }
    } catch {
      // Ignore parse errors.
    }
  }

  private save(): void {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.data));
    } catch {
      // Ignore storage errors.
    }
  }
}

// CSS class names for each theme applied to <body>.
export const THEME_CLASSES: Record<BoardTheme, string> = {
  green: "theme-green",
  brown: "theme-brown",
  blue: "theme-blue",
  grey: "theme-grey",
};

export function applyTheme(theme: BoardTheme): void {
  document.body.classList.remove(...Object.values(THEME_CLASSES));
  document.body.classList.add(THEME_CLASSES[theme]);
}
