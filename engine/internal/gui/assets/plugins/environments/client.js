// The Environments page (AgentDock): this computer and each WSL distro side
// by side. Every row comes from /api/environments: whether the gateway
// listens on this computer, and — apart from it — whether each running
// distro reaches it, as a probe run inside the distro said (a dial from
// Windows alone can't tell: Windows' firewall may still keep WSL out). A
// stopped distro is never started to ask. From here a distro is turned
// off or on, one agent's setup is copied onto its twins in the other
// environments (previewed first), .wslconfig is switched to mirrored
// networking, and providers come in from CC Switch or a distro's own
// magpie.
//
// Classic script, loaded before app.js; mount runs at magpie.start.
magpie.plugin("environments", (ctx) => {
  ctx.slots.add("page", {
    key: "environments",
    title: "Environments",
    html: `
    <div class="row-head"><span class="label" data-t>Environments</span><span class="note" data-t>this computer and each WSL distro</span><span class="grow"></span>
      <button class="text" id="envRefresh" type="button"><span data-t>Check again</span></button></div>
    <div id="envNotices"></div>
    <section class="list env-list" id="envList" aria-live="polite"></section>
    <div class="row-head"><span class="label" data-t>Copy an agent's setup</span><span class="note" data-t>onto the same agent in other environments</span></div>
    <section class="list env-sync" id="envSync"></section>
    <div class="row-head"><span class="label" data-t>WSL networking</span></div>
    <section class="list" id="envNet"></section>
    <div class="row-head"><span class="label" data-t>Bring in providers</span></div>
    <section class="list" id="envImport"></section>
`,
    mount() {
      const ready = document.readyState === "loading" ? new Promise((go) => document.addEventListener("DOMContentLoaded", go, { once: true })) : Promise.resolve();
      return ready.then(() => { envPage = environmentsPage(ctx); });
    },
    open() { return envPage?.load(); },
    relang() { envPage?.draw(); },
  });
});

let envPage = null;

function environmentsPage(ctx) {
  const ui = ctx.ui;
  let s = null; // /api/environments
  let sources = []; // a distro's own magpie
  let sync = { source: "", targets: [], plan: null, busy: false };
  let polling = 0;

  const agentName = (id) => (state.agents || []).find((a) => a.id === id)?.name || id;
  const kind = (id) => id.split("@")[0];
  const envOf = (id) => (id.includes("@wsl:") ? "wsl:" + id.split("@wsl:")[1] : "windows");
  const envName = (e) => (e.kind === "windows" ? t("This computer") : "WSL · " + e.distro);

  async function load() {
    try {
      s = await api("environments");
    } catch (e) {
      status(e.message, "err");
      return;
    }
    api("environments/import/magpie").then((r) => { sources = r.sources || []; drawImport(); }, () => {});
    draw();
    // a probe started in the background: read again once it is done
    clearTimeout(polling);
    if (s.probing && !$("#view-environments").hidden) polling = setTimeout(load, 2500);
  }

  function draw() {
    if (!s) return;
    drawNotices();
    drawList();
    drawSync();
    drawNet();
    drawImport();
  }

  // ---------- notices: another magpie, the gateway held elsewhere ----------

  function drawNotices() {
    const box = $("#envNotices");
    box.replaceChildren();
    const say = (text, kindOf = "warn") => {
      const n = el("div", "env-notice " + kindOf, text);
      n.setAttribute("role", "status");
      box.append(n);
    };
    if (s.heldElsewhere && s.holder) {
      say(t("Another magpie ({version}) serves the gateway at {url}: agents set up here reach that one. Quit it, or take the gateway over on the Gateway page.", { version: s.holder.version || "?", url: s.gateway }));
    } else if (s.heldElsewhere) {
      say(t("Something else holds the gateway's address {url} (another magpie behind a key, a container or a forward): agents set up here reach that, not AgentDock. Stop it, or move AgentDock's gateway to another port in Settings.", { url: s.gateway }));
    }
    if (s.others?.length) {
      say(t("Another magpie runs on this computer (pid {pids}): it writes the same agents' files. Quit it to keep AgentDock their only writer.", { pids: s.others.map((o) => o.pid).join(", ") }));
    }
    for (const e of s.environments) {
      if (e.reach?.magpie?.length) say(t("A magpie of its own runs in WSL {distro} (pid {pids}): it writes the same agents' files there. Quit it there.", { distro: e.distro, pids: e.reach.magpie.join(", ") }));
    }
    if (!s.looked) say(t("Detect agents in WSL is off in Settings: no distro is looked in."), "info");
  }

  // ---------- the environments ----------

  function reachPill(e) {
    const r = e.reach || {};
    const words = {
      ok: t("reaches the gateway"),
      blocked: t("can't reach the gateway"),
      unknown: t("couldn't tell"),
      "not-probed": e.state === "asleep" ? t("not running") : e.enabled === false ? t("turned off") : t("not checked yet"),
    };
    const p = el("span", "env-pill " + (r.status || "not-probed"));
    p.append(el("i"), el("span", "", words[r.status] || words["not-probed"]));
    const tip = [r.detail, r.url && t("asked {url} from inside the distro", { url: r.url }), r.checkedAt && new Date(r.checkedAt).toLocaleTimeString(intlLang())].filter(Boolean);
    if (tip.length) p.title = tip.join("\n");
    return p;
  }

  function drawList() {
    const box = $("#envList");
    box.replaceChildren();
    for (const e of s.environments) {
      const row = el("div", "row env-row");
      row.dataset.env = e.id;
      const who = el("div", "who");
      who.append(el("div", "name", envName(e)));
      const sub = [];
      if (e.kind === "windows") {
        sub.push(t("{n} agents", { n: (e.hostAgents || []).length }));
        sub.push(t("gateway at {url}", { url: s.gateway }));
      } else {
        sub.push(e.state === "running" ? t("running") : t("not running"));
        if (e.net) sub.push(t("{mode} networking", { mode: e.net }));
        else if (e.known) sub.push(e.mirrored ? t("mirrored networking (.wslconfig)") : t("networking unknown"));
        if (e.enabled) sub.push(t("{n} agents", { n: (e.agents || []).length }));
      }
      who.append(el("div", "sub", sub.join(" · ")));
      row.append(who);
      if (e.kind === "windows") {
        const p = el("span", "env-pill " + (e.hostListener ? "ok" : "blocked"));
        p.append(el("i"), el("span", "", e.hostListener ? t("gateway listening here") : t("gateway not listening here")));
        p.title = t("A connection from this computer only: whether a WSL distro reaches it is on its own row");
        row.append(p);
      } else {
        row.append(reachPill(e));
        if (e.reach?.status === "blocked" && s.environments[0].hostListener) {
          const why = el("div", "sub env-why", e.mirrored
            ? t("The gateway listens on this computer but the distro can't reach it: Windows' firewall may keep WSL out. Allow port {port} for the WSL network.", { port: s.gateway.split(":").pop() })
            : t("Under NAT networking the distro reaches Windows at another address. Switch WSL to mirrored networking below, or turn on Settings › Share on local network."));
          who.append(why);
        }
        const sw = ui.toggle(!!e.enabled, t("Manage WSL {distro}", { distro: e.distro }), async (on) => {
          try {
            await api("environments/" + encodeURIComponent(e.distro) + "/enabled", { enabled: on });
            // what the distro offers (its magpie's providers) comes and goes with it
            await load();
          } catch (err) { status(err.message, "err"); load(); }
        });
        if (!s.looked) sw.disabled = true;
        sw.title = t("Off, magpie neither lists nor writes anything in this distro");
        row.append(sw);
      }
      box.append(row);
    }
  }

  // ---------- copy an agent's setup ----------

  // every agent in any environment, by id, with its environment
  function allAgents() {
    const out = [];
    for (const e of s.environments) {
      for (const id of e.kind === "windows" ? e.hostAgents || [] : e.agents || []) out.push({ id, env: e });
    }
    return out;
  }

  function drawSync() {
    const box = $("#envSync");
    box.replaceChildren();
    const all = allAgents();
    // only an agent that is somewhere else too can be copied
    const twins = (id) => all.filter((a) => a.id !== id && kind(a.id) === kind(id));
    const froms = all.filter((a) => twins(a.id).length);
    if (!froms.length) {
      box.append(el("div", "row", el("div", "sub", t("No agent is in more than one environment yet."))));
      return;
    }
    if (!froms.some((a) => a.id === sync.source)) sync = { source: froms[0].id, targets: [], plan: null, busy: false };
    const pick = ui.menu(froms.map((a) => ({ v: a.id, name: agentName(kind(a.id)) + " · " + envName(a.env) })), sync.source, t("Copy from"), (v) => {
      sync = { source: v, targets: [], plan: null, busy: false };
      drawSync();
    });
    box.append(ui.row(t("Copy from"), t("Whether it goes through magpie, its model and other picks, and what the Library gives it. Sign-ins stay each environment's own."), pick));
    const chips = el("div", "env-chips");
    for (const a of twins(sync.source)) {
      const on = sync.targets.includes(a.id);
      const c = el("button", "chip" + (on ? " on" : ""), envName(a.env));
      c.type = "button";
      c.setAttribute("aria-pressed", String(on));
      c.onclick = () => {
        sync.targets = on ? sync.targets.filter((x) => x !== a.id) : [...sync.targets, a.id];
        sync.plan = null;
        drawSync();
      };
      chips.append(c);
    }
    box.append(ui.row(t("Onto"), "", chips));
    const acts = el("div", "env-acts");
    const preview = ui.button(t("Preview changes"), () => run("preview"));
    preview.disabled = !sync.targets.length || sync.busy;
    const apply = ui.button(t("Apply"), () => run("apply"), "text primary");
    apply.disabled = !sync.plan || sync.busy || !sync.plan.targets.some((x) => !x.skip && x.changes.length);
    acts.append(preview, apply);
    box.append(acts);
    if (sync.plan) box.append(planView(sync.plan));
  }

  function changeText(c) {
    switch (c.what) {
      case "connect": return t("connect it to magpie");
      case "disconnect": return t("disconnect it from magpie");
      case "field": return t("{field}: {from} → {to}", { field: c.key, from: c.from || t("(its own)"), to: c.to || t("(its own)") });
      case "library": {
        const [k, n] = c.key.split(":");
        const what = { instructions: t("shared instructions"), mcp: t("MCP server {name}"), skill: t("skill {name}"), hook: t("hook {name}"), subagent: t("subagent {name}") }[k] || c.key;
        return (c.to === "on" ? "+ " : "− ") + what.replace("{name}", n || "");
      }
    }
    return c.what;
  }

  function planView(p) {
    const box = el("div", "env-plan");
    for (const tp of p.targets) {
      const card = el("div", "env-plan-target");
      card.append(el("div", "name", agentName(kind(tp.agent)) + " · " + envName(s.environments.find((e) => e.id === envOf(tp.agent)) || { kind: "wsl", distro: tp.agent })));
      if (tp.skip) card.append(el("div", "sub warn", t("Skipped: {why}", { why: tp.skip })));
      else if (!tp.changes.length) card.append(el("div", "sub", t("Nothing to change")));
      else {
        const ul = el("ul");
        for (const c of tp.changes) ul.append(el("li", "", changeText(c)));
        card.append(ul);
      }
      if (tp.error) card.append(el("div", "sub warn", tp.error));
      box.append(card);
    }
    for (const pr of p.problems || []) box.append(el("div", "sub warn", t("{agent} couldn't be given {what}: {why}", { agent: pr.agent, what: pr.what, why: pr.error })));
    return box;
  }

  async function run(how) {
    if (how === "apply" && !(await confirmAction(t("Copy the setup"), t("The changes listed are written into each agent's files there. Go on?"), t("Apply")))) return;
    sync.busy = true;
    drawSync();
    try {
      const p = await api("environments/sync/" + how, { source: sync.source, targets: sync.targets });
      sync.plan = how === "preview" ? p : null;
      if (how === "apply") {
        const failed = p.targets.filter((x) => x.error).length + (p.problems || []).length;
        status(failed ? t("Copied, with {n} problems", { n: failed }) : t("Copied"), failed ? "err" : "");
        sync.plan = p;
      }
    } catch (e) {
      status(e.message, "err");
    }
    sync.busy = false;
    if (how === "apply") await load();
    drawSync();
  }

  // ---------- WSL networking ----------

  function drawNet() {
    const box = $("#envNet");
    box.replaceChildren();
    const c = s.wslconfig;
    if (!c) {
      box.append(el("div", "row", el("div", "sub", t("No WSL on this computer."))));
      return;
    }
    const mode = c.mirrored ? t("mirrored") : c.networkingMode || t("NAT (WSL's default)");
    const say = [t("{path}: {mode} networking", { path: c.path, mode })];
    if (c.localhostForwardingOff) say.push(t("localhostForwarding=false turns mirrored's loopback off"));
    const acts = el("div", "env-acts");
    const mirror = ui.button(t("Switch to mirrored"), switchMirrored);
    mirror.disabled = c.mirrored || !c.supported;
    if (!c.supported) mirror.title = t("Mirrored networking needs Windows 11 22H2 (build 22621) or later");
    const restore = ui.button(t("Put the last copy back"), restoreConfig);
    restore.disabled = !c.backups?.length;
    const shut = ui.button(t("Restart WSL"), shutdownWSL);
    acts.append(mirror, restore, shut);
    box.append(ui.row(t("Networking"), say.join(" · "), acts));
  }

  async function switchMirrored() {
    if (!(await confirmAction(t("Switch WSL to mirrored networking"), t("networkingMode=mirrored is written under [wsl2] in .wslconfig, and the file as it is now is kept beside it. WSL reads it once restarted."), t("Switch")))) return;
    try {
      const r = await api("environments/wslconfig/mirrored", {});
      status(r.backup ? t("Written; the old file is kept as {path}", { path: r.backup }) : t("Written"));
      await load();
      if (r.running?.length) shutdownWSL(r.running);
    } catch (e) { status(e.message, "err"); }
  }

  async function restoreConfig() {
    if (!(await confirmAction(t("Put .wslconfig back"), t("The last copy AgentDock kept goes back in place; the file as it is now is kept too."), t("Put back")))) return;
    try {
      const r = await api("environments/wslconfig/restore", {});
      status(t("Put back from {path}", { path: r.restored }));
      await load();
    } catch (e) { status(e.message, "err"); }
  }

  // wsl --shutdown stops every distro: they are named first, and the user
  // confirms once more
  async function shutdownWSL(known) {
    let running = known;
    if (!Array.isArray(running)) {
      try { running = (await api("environments/wsl/running")).running; } catch (e) { status(e.message, "err"); return; }
    }
    const names = running.length ? running.join(", ") : t("none");
    if (!(await confirmAction(t("Restart WSL"), t("wsl --shutdown stops every running distro now ({names}), with what runs in them: open sessions and Docker Desktop too. They read .wslconfig when they start again.", { names }), t("Shut down")))) return;
    try {
      await api("environments/wsl/shutdown", { confirm: true });
      status(t("WSL shut down"));
      await load();
    } catch (e) { status(e.message, "err"); }
  }

  // ---------- bring in providers ----------

  function drawImport() {
    const box = $("#envImport");
    if (!box || !s) return;
    box.replaceChildren();
    if (s.firstRun) box.append(el("div", "env-notice info", t("No providers here yet: bring in the ones you already set up.")));
    const cc = ui.button(t("From CC Switch…"), async () => { await show("providers"); openImportApps(); });
    box.append(ui.row(t("CC Switch, Codex, Claude Code and others"), t("The providers those apps have, picked one by one"), cc));
    for (const src of sources) {
      const b = ui.button(t("Bring in"), async () => {
        try {
          const r = await api("environments/import/magpie", { distro: src.distro });
          const n = r.added.length + Object.keys(r.renamed).length + r.groups.length;
          const skipped = Object.entries(r.skipped).map(([k, v]) => k + ": " + v);
          status(t("{n} brought in from WSL {distro}", { n, distro: src.distro }) + (skipped.length ? " · " + t("left out: {list}", { list: skipped.join("; ") }) : ""), skipped.length ? "err" : "");
          await load();
          loadProviders?.();
        } catch (e) { status(e.message, "err"); }
      });
      if (src.error) b.disabled = true;
      const sub = src.error || t("{p} providers, {g} routing groups; one here already with other settings comes in as <id>-wsl", { p: src.providers.length, g: src.groups.length });
      box.append(ui.row(t("magpie in WSL {distro}", { distro: src.distro }), sub, b));
    }
  }

  $("#envRefresh").onclick = async () => {
    const b = $("#envRefresh");
    b.disabled = true;
    try { s = await api("environments/reach/refresh", {}); draw(); } catch (e) { status(e.message, "err"); }
    b.disabled = false;
  };

  return { load, draw };
}
