import {
  F_,
  K_,
  Q_,
  fe,
  normalizeOutputPad,
  appearancePresets,
  currentAppearance,
  appearancePresetLabels,
  persistAppearance,
  subscribeAppearance
} from "./app-z959mk0b.js";

// web/src/gi-settings-appearance.ts
function Appearance() {
  const [value, setValue] = F_(currentAppearance);
  const [error, setError] = F_("");
  const ownSave = Q_(false);
  K_(() => subscribeAppearance((next) => {
    if (!ownSave.current)
      setValue(next);
  }), []);
  const apply = (patch) => {
    const next = { ...value, ...patch };
    setError("");
    ownSave.current = true;
    try {
      setValue(persistAppearance(next));
    } catch (err) {
      setError(`Appearance was not saved: ${err.message}`);
      setValue((previous) => ({ ...previous }));
    } finally {
      ownSave.current = false;
    }
  };
  const pad = normalizeOutputPad(value.outputPad);
  return fe`<section aria-labelledby="gi-appearance-title" class="settings-section settings-appearance">
        <h2 id="gi-appearance-title">Appearance</h2>
        <p>Browser settings · this origin, across all sessions. Changes apply at once and stay in this browser profile.</p>
        ${error && fe`<p role="alert">${error}</p>`}
        <div class="settings-tint-row">
            <label class="settings-tint-label">
                <input type="radio" name="gi-settings-theme" checked=${value.theme === "default"} onChange=${() => apply({ theme: "default" })} />
                <strong>Default</strong> <span class="settings-hint">auto light/dark</span>
            </label>
            <div class="settings-tint-picker">
                <label class="settings-hint" for="gi-settings-tint">Tint</label>
                <input id="gi-settings-tint" type="color" value=${value.tint || "#1d9bf0"}
                    onInput=${(e) => apply({ theme: "default", tint: e.target.value })} />
                ${value.tint && fe`<button class="settings-tint-clear" title="Clear tint" aria-label="Clear tint" onClick=${() => apply({ theme: "default", tint: "" })}>✕</button>`}
                <span class="settings-tint-hex">${value.tint || "none"}</span>
            </div>
        </div>
        <div class="settings-output-pad-row">
            <label class="settings-output-pad-label" for="gi-settings-output-pad"><strong>Output padding</strong>
                <span class="settings-hint">Extra space around timeline posts.</span></label>
            <div class="settings-output-pad-control">
                <input id="gi-settings-output-pad" type="range" min="0" max="24" step="1" value=${pad} onInput=${(e) => apply({ outputPad: normalizeOutputPad(e.target.value) })} />
                <input class="settings-output-pad-number" aria-label="Output padding (px)" type="number" min="0" max="24" step="1" value=${pad} onInput=${(e) => apply({ outputPad: normalizeOutputPad(e.target.value) })} />
                <span class="settings-hint">px</span>
            </div>
        </div>
        <table class="settings-table settings-borderless settings-theme-table">
            <thead><tr><th scope="col"><span class="gi-visually-hidden">Selected</span></th><th scope="col">Theme</th></tr></thead>
            <tbody>
                ${appearancePresets.filter((name) => name !== "default").map((name) => fe`<tr class=${name === value.theme ? "settings-row-active" : ""} style="cursor:pointer" onClick=${() => apply({ theme: name, tint: "" })}>
                    <td><input type="radio" name="gi-settings-theme" aria-label=${appearancePresetLabels[name] || name} checked=${name === value.theme} onChange=${() => apply({ theme: name, tint: "" })} /></td>
                    <td><strong>${appearancePresetLabels[name] || name}</strong></td>
                </tr>`)}
            </tbody>
        </table>
    </section>`;
}
export {
  Appearance
};
