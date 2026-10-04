import {
  F_,
  K_,
  Q_,
  u_,
  fe,
  getEnvironmentSettings,
  setEnvironmentOverride,
  clearEnvironmentOverride
} from "./app-0y8n023b.js";

// web/src/gi-settings-environment.ts
var NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;
function GiSettingsEnvironment() {
  const [data, setData] = F_(null);
  const [drafts, setDrafts] = F_({});
  const [name, setName] = F_("");
  const [value, setValue] = F_("");
  const [filter, setFilter] = F_("");
  const [error, setError] = F_("");
  const [announcement, setAnnouncement] = F_("");
  const alive = Q_(true);
  K_(() => () => {
    alive.current = false;
  }, []);
  const apply = (result, message) => {
    if (!alive.current)
      return false;
    if (result.status >= 200 && result.status < 300 && Array.isArray(result.data?.variables)) {
      setData(result.data);
      setError("");
      setAnnouncement(message);
      return true;
    }
    setError(result.data?.error || "Failed to update the environment.");
    return false;
  };
  K_(() => {
    getEnvironmentSettings().then((r) => apply(r, ""), () => alive.current && setError("Failed to load the environment."));
  }, []);
  async function add() {
    const n = name.trim();
    if (!NAME.test(n)) {
      setError("Variable names must be shell identifiers (letters, digits, _; not starting with a digit).");
      return;
    }
    if (apply(await setEnvironmentOverride(n, value), `${n} saved.`)) {
      setName("");
      setValue("");
      setDrafts((d) => {
        const { [n]: _, ...rest } = d;
        return rest;
      });
    }
  }
  async function save(n) {
    if (apply(await setEnvironmentOverride(n, drafts[n] ?? ""), `${n} saved.`))
      setDrafts((d) => {
        const { [n]: _, ...rest } = d;
        return rest;
      });
  }
  async function clear(n) {
    if (apply(await clearEnvironmentOverride(n), `${n} override cleared.`))
      setDrafts((d) => {
        const { [n]: _, ...rest } = d;
        return rest;
      });
  }
  const query = filter.trim().toLowerCase();
  const rows = u_(() => (data?.variables || []).filter((v) => !query || v.name.toLowerCase().includes(query)), [data, query]);
  return fe`<section aria-labelledby="gi-environment-title" class="gi-environment">
        <h2 id="gi-environment-title">Environment</h2>
        <p>Variables every shell command inherits. An override applies to the next command and is kept across restarts; clearing it restores the inherited value. Keychain variables are managed under Keychain.</p>
        <div role="status" aria-live="polite" class="gi-keychain-announcement">${announcement}</div>
        ${error && fe`<p role="alert">${error} <button aria-label="Dismiss environment error" onClick=${() => setError("")}>✕</button></p>`}
        <div class="gi-environment-add" role="group" aria-label="Add environment override">
            <input type="text" aria-label="Variable name" placeholder="Variable name" autocomplete="off" spellcheck="false" value=${name} onInput=${(e) => setName(e.target.value)} />
            <input type="text" aria-label="Value" placeholder="Value" autocomplete="off" spellcheck="false" value=${value} onInput=${(e) => setValue(e.target.value)}
                onKeyDown=${(e) => {
    if (e.key === "Enter")
      add();
  }} />
            <button onClick=${add} disabled=${!name.trim()}>Add</button>
        </div>
        ${data === null && !error && fe`<p role="status">Loading environment…</p>`}
        ${data !== null && fe`
            <input type="search" class="gi-keychain-filter" aria-label="Filter variables" placeholder="Filter variables…" value=${filter} onInput=${(e) => setFilter(e.target.value)} />
            <p class="gi-keychain-count">${data.count} variables, ${data.overrideCount} overridden.</p>
            <div class="gi-keychain-table-wrap"><table class="gi-keychain-table gi-environment-table">
                <thead><tr><th>Name</th><th>Value</th><th><span class="gi-visually-hidden">Actions</span></th></tr></thead>
                <tbody>
                    ${rows.map((v) => fe`<tr key=${v.name} class="gi-environment-row">
                        <td class="gi-keychain-name"><span>${v.name}</span>${v.overridden && fe` <span class="gi-keychain-type">override</span>`}</td>
                        <td><input type="text" aria-label=${`Value of ${v.name}`} autocomplete="off" spellcheck="false" value=${drafts[v.name] ?? v.value}
                            onInput=${(e) => setDrafts((d) => ({ ...d, [v.name]: e.target.value }))}
                            onKeyDown=${(e) => {
    if (e.key === "Enter")
      save(v.name);
  }} /></td>
                        <td class="gi-keychain-actions">
                            <button aria-label="Save" title=${`Save ${v.name}`} onClick=${() => save(v.name)}>Save</button>
                            ${v.overridden && fe`<button aria-label="Clear" title=${`Clear the ${v.name} override`} onClick=${() => clear(v.name)}>Clear</button>`}
                        </td>
                    </tr>`)}
                </tbody>
            </table></div>`}
    </section>`;
}
export {
  GiSettingsEnvironment
};
