import {
  F_,
  K_,
  W_,
  Q_,
  u_,
  fe,
  listKeychain,
  saveKeychainEntry,
  deleteKeychainEntry,
  revealKeychainEntry
} from "./app-hy4by8xf.js";

// web/src/gi-settings-keychain.ts
var TYPES = ["secret", "token", "password", "basic"];
function formatDate(iso) {
  if (!iso)
    return "—";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}
async function copy(text) {
  try {
    await navigator.clipboard.writeText(text);
  } catch {
    const area = document.createElement("textarea");
    area.value = text;
    area.style.position = "fixed";
    area.style.opacity = "0";
    document.body.appendChild(area);
    area.select();
    document.execCommand("copy");
    area.remove();
  }
}
var emptyDraft = { name: "", type: "secret", secret: "", username: "", userNote: "", agentNote: "" };
function GiSettingsKeychain() {
  const [entries, setEntries] = F_(null);
  const [enabled, setEnabled] = F_(true);
  const [error, setError] = F_("");
  const [announcement, setAnnouncement] = F_("");
  const [filter, setFilter] = F_("");
  const [adding, setAdding] = F_(false);
  const [draft, setDraft] = F_(emptyDraft);
  const [saving, setSaving] = F_(false);
  const [confirmDelete, setConfirmDelete] = F_("");
  const [reveal, setReveal] = F_(null);
  const alive = Q_(true);
  const nameRef = Q_(null);
  const passwordRef = Q_(null);
  K_(() => () => {
    alive.current = false;
  }, []);
  async function load() {
    try {
      const { data } = await listKeychain();
      if (!alive.current)
        return;
      if (data?.ok) {
        setEntries(data.entries || []);
        setEnabled(data.enabled !== false);
      } else
        setError(data?.error || "Failed to load keychain.");
    } catch {
      if (alive.current)
        setError("Failed to load keychain.");
    }
  }
  K_(() => {
    load();
  }, []);
  W_(() => {
    if (adding)
      nameRef.current?.focus();
  }, [adding]);
  W_(() => {
    if (reveal?.phase === "password")
      passwordRef.current?.focus();
  }, [reveal?.name, reveal?.phase]);
  async function add() {
    const name = draft.name.trim();
    if (!name || !draft.secret || saving)
      return;
    setSaving(true);
    setError("");
    try {
      const { data } = await saveKeychainEntry({ ...draft, name, username: draft.username.trim() || undefined });
      if (!alive.current)
        return;
      if (data?.ok) {
        setDraft(emptyDraft);
        setAdding(false);
        setAnnouncement("Keychain entry saved.");
        await load();
      } else
        setError(data?.error || "Failed to add entry.");
    } catch {
      if (alive.current)
        setError("Failed to add entry.");
    } finally {
      if (alive.current)
        setSaving(false);
    }
  }
  async function remove(name) {
    setError("");
    try {
      const { data } = await deleteKeychainEntry(name);
      if (!alive.current)
        return;
      if (data?.ok) {
        setConfirmDelete("");
        setReveal((r) => r?.name === name ? null : r);
        setAnnouncement("Keychain entry deleted.");
        await load();
      } else
        setError(data?.error || "Failed to delete entry.");
    } catch {
      if (alive.current)
        setError("Failed to delete entry.");
    }
  }
  async function doReveal(name, password) {
    try {
      const { data } = await revealKeychainEntry(name, password);
      if (!alive.current)
        return;
      if (data?.ok) {
        setReveal({ name, phase: "revealed", secret: data.secret, username: data.username });
        setAnnouncement("Secret revealed.");
      } else if (data?.needs_master_password) {
        setReveal({ name, phase: "password", password: "", error: password ? data.error : "" });
      } else
        setReveal({ name, phase: "error", error: data?.error || "Failed to reveal." });
    } catch {
      if (alive.current)
        setReveal({ name, phase: "error", error: "Failed to reveal." });
    }
  }
  const toggleReveal = (name) => reveal?.name === name && reveal.phase === "revealed" ? setReveal(null) : doReveal(name);
  const query = filter.trim().toLowerCase();
  const shown = u_(() => (entries || []).filter((e) => !query || [e.name, e.type, e.envVar, e.userNote, e.agentNote].some((v) => String(v || "").toLowerCase().includes(query))), [entries, query]);
  const count = `${shown.length} ${shown.length === 1 ? "entry" : "entries"}${query ? ` matching "${filter.trim()}"` : ""}, encrypted at rest.`;
  return fe`<section aria-labelledby="gi-keychain-title" class="gi-keychain">
        <h2 id="gi-keychain-title">Keychain</h2>
        <p>Credentials encrypted in this instance's database. A shell command that names an entry's variable ($NAME or \${NAME}) gets its secret; <code>keychain:&lt;name&gt;</code> in a command is replaced by it.</p>
        <div role="status" aria-live="polite" class="gi-keychain-announcement">${announcement}</div>
        ${!enabled && fe`<p role="alert">Keychain is disabled. Start Gi with GI_KEYCHAIN_KEY or GI_KEYCHAIN_KEY_FILE set to add or reveal entries.</p>`}
        ${error && fe`<p role="alert">${error} <button aria-label="Dismiss keychain error" onClick=${() => setError("")}>✕</button></p>`}
        ${entries === null && !error && fe`<p role="status">Loading keychain…</p>`}
        ${entries !== null && fe`
            <div class="gi-keychain-toolbar">
                <input type="search" class="gi-keychain-filter" aria-label="Filter entries" placeholder="Filter entries…" value=${filter} onInput=${(e) => setFilter(e.target.value)} />
                <button aria-expanded=${adding} onClick=${() => setAdding(!adding)}>${adding ? "Cancel" : "+ Add entry"}</button>
            </div>
            <p class="gi-keychain-count">${count}</p>
            ${adding && fe`<div class="gi-keychain-add" role="group" aria-label="Add keychain entry" aria-busy=${saving}>
                <input ref=${nameRef} type="text" aria-label="Entry name" placeholder="Entry name (e.g. github/my-token)" value=${draft.name} onInput=${(e) => setDraft((d) => ({ ...d, name: e.target.value }))} />
                <select aria-label="Entry type" value=${draft.type} onChange=${(e) => setDraft((d) => ({ ...d, type: e.target.value }))}>
                    ${TYPES.map((t) => fe`<option value=${t}>${t}</option>`)}
                </select>
                <input type="password" aria-label="Entry secret" autocomplete="off" placeholder="Secret value" value=${draft.secret} onInput=${(e) => setDraft((d) => ({ ...d, secret: e.target.value }))} />
                <input type="text" aria-label="Entry username" placeholder="Username (optional)" value=${draft.username} onInput=${(e) => setDraft((d) => ({ ...d, username: e.target.value }))} />
                <textarea aria-label="User note" rows="2" placeholder="User note (visible in this UI only)" value=${draft.userNote} onInput=${(e) => setDraft((d) => ({ ...d, userNote: e.target.value }))}></textarea>
                <textarea aria-label="Agent note" rows="2" placeholder="Agent note (safe to expose to agents)" value=${draft.agentNote} onInput=${(e) => setDraft((d) => ({ ...d, agentNote: e.target.value }))}></textarea>
                <button onClick=${add} disabled=${saving || !draft.name.trim() || !draft.secret}>${saving ? "Saving…" : "Save"}</button>
            </div>`}
            <div class="gi-keychain-table-wrap"><table class="gi-keychain-table">
                <thead><tr><th>Name</th><th>Type</th><th>Env var</th><th>Updated</th><th><span class="gi-visually-hidden">Actions</span></th></tr></thead>
                <tbody>
                    ${shown.length === 0 && fe`<tr><td colspan="5" class="gi-keychain-empty">${query ? "No entries match the filter." : "No keychain entries."}</td></tr>`}
                    ${shown.map((e) => {
    const r = reveal?.name === e.name ? reveal : null;
    const revealed = r?.phase === "revealed";
    return fe`<tr key=${e.name} class="gi-keychain-row">
                            <td class="gi-keychain-name">${e.name}</td>
                            <td><span class="gi-keychain-type">${e.type}</span></td>
                            <td>${e.envVar ? fe`<code>$${e.envVar}</code>` : "—"}</td>
                            <td>${formatDate(e.updatedAt)}</td>
                            <td class="gi-keychain-actions">
                                <button aria-label=${`${revealed ? "Hide secret" : "Reveal secret"}: ${e.name}`} aria-expanded=${Boolean(r)} onClick=${() => toggleReveal(e.name)}>${revealed ? "Hide" : "Reveal"}</button>
                                ${confirmDelete === e.name ? fe`<span class="gi-keychain-confirm">Delete?
                                        <button aria-label=${`Delete ${e.name}`} onClick=${() => remove(e.name)}>Yes</button>
                                        <button onClick=${() => setConfirmDelete("")}>No</button></span>` : fe`<button aria-label=${`Delete ${e.name}`} onClick=${() => setConfirmDelete(e.name)}>Delete</button>`}
                            </td>
                        </tr>
                        ${r?.phase === "password" && fe`<tr key=${e.name + "-pw"} class="gi-keychain-detail"><td colspan="5">
                            <label>Master password<input ref=${passwordRef} type="password" autocomplete="off" aria-label="Master password" placeholder="Enter keychain master password" value=${r.password}
                                onInput=${(ev) => setReveal((s) => ({ ...s, password: ev.target.value }))}
                                onKeyDown=${(ev) => {
      if (ev.key === "Enter" && r.password)
        doReveal(e.name, r.password);
    }} /></label>
                            <button disabled=${!r.password} onClick=${() => doReveal(e.name, r.password)}>Unlock</button>
                            <button onClick=${() => setReveal(null)}>Cancel</button>
                            ${r.error && fe`<span role="alert">${r.error}</span>`}
                        </td></tr>`}
                        ${revealed && fe`<tr key=${e.name + "-secret"} class="gi-keychain-detail"><td colspan="5">
                            ${r.username && fe`<div class="gi-keychain-field"><span>Username</span><code>${r.username}</code><button aria-label="Copy username" onClick=${() => copy(r.username)}>Copy</button></div>`}
                            <div class="gi-keychain-field"><span>Secret</span><code>${r.secret}</code><button aria-label="Copy secret" onClick=${() => copy(r.secret)}>Copy</button></div>
                        </td></tr>`}
                        ${r?.phase === "error" && fe`<tr key=${e.name + "-error"} class="gi-keychain-detail"><td colspan="5"><span role="alert">${r.error}</span></td></tr>`}`;
  })}
                </tbody>
            </table></div>`}
    </section>`;
}
export {
  GiSettingsKeychain
};
