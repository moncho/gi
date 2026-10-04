import {
  F_,
  K_,
  u_,
  fe,
  useTranslation,
  KEYBOARD_SHORTCUT_ACTIONS,
  normalizeShortcutBindingString,
  parseShortcutBindingList,
  formatShortcutBindingList,
  saveKeyboardShortcutBindings,
  resetKeyboardShortcutBindings,
  readAllKeyboardShortcutBindings
} from "./app-dbj801ab.js";

// web/src/ui/keyboard-shortcut-settings.ts
function readKeyboardShortcutDrafts() {
  const current = readAllKeyboardShortcutBindings();
  return Object.fromEntries(Object.entries(current).map(([id, bindings]) => [id, formatShortcutBindingList(bindings)]));
}
function filterKeyboardShortcutActions(filter, drafts) {
  const query = String(filter || "").trim().toLowerCase();
  if (!query)
    return KEYBOARD_SHORTCUT_ACTIONS;
  return KEYBOARD_SHORTCUT_ACTIONS.filter((action) => [
    action.label,
    action.description,
    drafts[action.id],
    ...action.defaultBindings
  ].some((part) => String(part || "").toLowerCase().includes(query)));
}
function saveKeyboardShortcutDraft(actionId, rawValue) {
  const raw = String(rawValue || "").trim();
  const tokens = raw ? raw.split(/[\n,]/).map((part) => part.trim()).filter(Boolean) : [];
  const invalidToken = tokens.find((token) => !normalizeShortcutBindingString(token));
  if (invalidToken)
    return { ok: false, invalidToken, drafts: readKeyboardShortcutDrafts() };
  saveKeyboardShortcutBindings(actionId, parseShortcutBindingList(raw));
  return { ok: true, drafts: readKeyboardShortcutDrafts() };
}
function resetKeyboardShortcutDraft(actionId) {
  resetKeyboardShortcutBindings(actionId);
  return readKeyboardShortcutDrafts();
}

// web/src/components/settings/keyboard.ts
function KeyboardSection({ filter = "", setStatus }) {
  const { t } = useTranslation();
  const [drafts, setDrafts] = F_(readKeyboardShortcutDrafts);
  K_(() => {
    const sync = () => setDrafts(readKeyboardShortcutDrafts());
    window.addEventListener("piclaw:keyboard-shortcuts-changed", sync);
    return () => window.removeEventListener("piclaw:keyboard-shortcuts-changed", sync);
  }, []);
  const visibleActions = u_(() => filterKeyboardShortcutActions(filter, drafts), [drafts, filter]);
  const saveAction = (actionId) => {
    const result = saveKeyboardShortcutDraft(actionId, drafts[actionId] || "");
    if (!result.ok) {
      setStatus?.(t("settings.keyboard.invalidShortcut", { token: result.invalidToken || "" }), "error");
      return;
    }
    setDrafts(result.drafts);
    setStatus?.(t("settings.keyboard.saved"), "success");
  };
  const resetAction = (actionId) => {
    setDrafts(resetKeyboardShortcutDraft(actionId));
    setStatus?.(t("settings.keyboard.resetOne"), "success");
  };
  const resetAll = () => {
    setDrafts(resetKeyboardShortcutDraft());
    setStatus?.(t("settings.keyboard.resetAllDone"), "success");
  };
  return fe`
        <div class="settings-section">
            <h3>${t("settings.keyboard.heading")}</h3>
            <p class="settings-hint">
                ${t("settings.keyboard.hint1")}
                <code>Escape</code> ${t("settings.keyboard.hint1b")}
            </p>
            <p class="settings-hint">
                <code>/help</code> ${t("settings.keyboard.hint2mid")} <code>"</code> ${t("settings.keyboard.hint2end")}
            </p>

            <div class="settings-shortcut-toolbar">
                <button type="button" class="settings-addon-btn" onClick=${resetAll}>${t("settings.keyboard.resetAll")}</button>
            </div>

            <div class="settings-shortcut-list">
                ${visibleActions.map((action) => {
    const inputId = `settings-shortcut-${action.id}`;
    return fe`
                    <div class="settings-shortcut-card" key=${action.id}>
                        <div class="settings-shortcut-copy">
                            <label class="settings-shortcut-title" for=${inputId}>${action.label}</label>
                            <div class="settings-hint settings-shortcut-description">${action.description}</div>
                            <div class="settings-shortcut-default">${t("settings.keyboard.defaultColon")} <code>${formatShortcutBindingList(action.defaultBindings)}</code></div>
                        </div>
                        <div class="settings-shortcut-controls">
                            <input
                                id=${inputId}
                                class="settings-shortcut-input"
                                type="text"
                                value=${drafts[action.id] || ""}
                                placeholder=${formatShortcutBindingList(action.defaultBindings)}
                                onInput=${(e) => setDrafts((prev) => ({ ...prev, [action.id]: e.target.value }))}
                            />
                            <div class="settings-shortcut-actions">
                                <button type="button" class="settings-addon-btn settings-addon-btn-install" onClick=${() => saveAction(action.id)}>${t("settings.keyboard.save")}</button>
                                <button type="button" class="settings-addon-btn" onClick=${() => resetAction(action.id)}>${t("settings.keyboard.defaultBtn")}</button>
                            </div>
                        </div>
                    </div>
                `;
  })}
                ${visibleActions.length === 0 && fe`<div class="settings-hint">${t("settings.keyboard.noMatch")}</div>`}
            </div>
        </div>
    `;
}
export {
  KeyboardSection
};
