// The credits read from commit subjects (credits.mjs). Every subject here is
// one from yetone/magpie's own history, word for word.
import test from "node:test";
import assert from "node:assert/strict";
import { parse, credit, everyone, thanks, what, wall, renderWall } from "./credits.mjs";
import { authors, excluded } from "./credits-cli.mjs";
import { mkdtempSync, writeFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const names = (subject, opts) => credit([{ subject, author: "yetone", email: "yetoneful@gmail.com" }], opts)[0].map((p) => p.name);
const via = (subject) => credit([{ subject, author: "yetone", email: "yetoneful@gmail.com" }])[0].map((p) => `${p.name}/${p.via.join("+")}`);

test("a reporter named on Discord or X is credited, with where", () => {
  assert.deepEqual(via("library: an MCP router says which computer it is, so an agent can pick between two (packinggoodsllc_07668 on Discord)"), ["packinggoodsllc_07668/Discord"]);
  assert.deepEqual(via("gateway: a HEIC photo goes to the vendor as JPEG, or PNG with alpha (qikamatata on X)"), ["qikamatata/X"]);
  assert.deepEqual(via("redact: a decimal's digits are not masked as a number of their own (willz, Discord)"), ["willz/Discord"]);
});

test("several people in one bracket are each credited", () => {
  assert.deepEqual(names("masking: a record names the provider the request went to, not the group's first member (Max You and Linka on Discord)"), ["Max You", "Linka"]);
  // the first is bare, so it needs naming elsewhere: here a commit that
  // credits TJHHHH on Discord
  const commits = [
    { subject: "codex: the restart advice after a model or catalog change names the app-server codex sessions share (\"codex app-server daemon restart\"), on Windows and WSL too (TJHHHH, luci on Discord)" },
    { subject: "codex: a model picked in the Codex app is the one it runs (TJHHHH on Discord)" },
  ];
  assert.deepEqual(credit(commits)[0].map((p) => p.name).sort(), ["TJHHHH", "luci"]);
  assert.deepEqual(credit(commits.slice(0, 1))[0].map((p) => p.name), ["luci"]);
});

test("a lone bracketed handle is credited when shaped like one or named elsewhere", () => {
  assert.deepEqual(names("guard: the local masking model reads text a third faster on amd64 (mrwl666)"), ["mrwl666"]);
  const lone = { subject: "gui: Busy models last takes a custom threshold, 1 to 9999 (axelburks)" };
  assert.deepEqual(credit([lone])[0], []);
  const both = credit([lone, { subject: "search: searchers picked to follow the one named, tried in their order when it fails (axelburks on Discord)" }]);
  assert.deepEqual(both[0].map((p) => p.name), ["axelburks"]);
});

test("a quote after the name is not part of it", () => {
  assert.deepEqual(names("zen: Test with the public key asks each API for a free model Zen serves on it, and leaves out an API no free model is on (z'x on Discord; #2385)"), ["z'x"]);
  assert.deepEqual(parse("zen: Test with the public key asks each API for a free model Zen serves on it, and leaves out an API no free model is on (z'x on Discord; #2385)").refs, [{ repo: "yetone/magpie", n: 2385 }]);
});

test("the maintainer, the owner and descriptions of someone are nobody", () => {
  assert.deepEqual(names("plugins: the GitHub list waits up to 3s for a repository's https icon fetched the first time (yetone: 非官方插件显示不出来 logo)"), []);
  assert.deepEqual(names("tray: with the menu bar's magpie icon off, a click opens the panel on the tab left last, not Allowances (the owner: 每次都会自动跳转到 allowances，我记得之前是可以的)"), []);
  assert.deepEqual(names("gui: a Claude account saved in magpie can be removed while Claude Code is signed out (𝕏 user on Discord: a banned account logged out, and its line in Providers had nothing to remove it with) — saved accounts are listed"), []);
});

test("code, labels and words beside an issue number are not people", () => {
  for (const s of [
    "plugin: gateway key limits are the Key limits plugin on the gateway's budget point (P3)",
    "routing: picking Live or a day keeps the page where it is, and a list with no requests says what it is for (Image #59) — a day or Live picked",
    "gui tests: the Routing page's tests pick a day from the date bar, not the By request button (#860 flake) — remote-group-served's zh case",
    "mcpauth: a sign-in asks for offline_access when the authorization server offers it, so Vercel's MCP stays signed in (#615: \"vercel老是掉\"; Neon, Notion and Supabase stay) — mcp.vercel.com's",
    "codex: a client's access_programs and the account's extraBody reach the ChatGPT backend, and its refusal comes back as it said it (magpie-releases#18)",
    "efd1b697e sessions: a cached file's calls are read into a slice made for them",
  ]) assert.deepEqual(names(s), [], s);
  // a word beside a credit is a person only when named somewhere else
  assert.deepEqual(names("agent: Claude Desktop on magpie keeps the language picked signed in with Anthropic (memory, Osal on Discord)"), ["Osal"]);
});

test("issue and PR numbers are kept for their authors, in either repository", () => {
  assert.deepEqual(parse("redact: a file named like an address (…@2x.jpg) is not masked as an email (#3029)").refs, [{ repo: "yetone/magpie", n: 3029 }]);
  assert.deepEqual(parse("redact: Codex's tool server names, stream_options and snake_case words go as written (#3143, #3144)").refs.map((r) => r.n), [3143, 3144]);
  assert.deepEqual(parse("provider: Devin's plugin min is 0.1.30, its opt-in cache keepalive (plugins#592)").refs, [{ repo: "magpie-community/plugins", n: 592 }]);
  assert.deepEqual(parse("provider: WorkBuddy's mover asks for opencode-workbuddy-auth 0.2.9, whose declared models take images (plugins #572)").refs, [{ repo: "magpie-community/plugins", n: 572 }]);
  const s = "redact: Codex's tool server names, stream_options and snake_case words go as written (#3143, #3144)";
  const issueAuthors = { "yetone/magpie#3143": "hhh2210", "yetone/magpie#3144": "dependabot[bot]" };
  const got = credit([{ subject: s, author: "yetone" }], { issueAuthors })[0];
  assert.deepEqual(got.map((p) => [p.name, p.github]), [["hhh2210", "hhh2210"]]);
  // a name and an issue in one bracket: both
  assert.deepEqual(names("provider: a Max output kept for a model no longer listed doesn't block the editor's Save (#3305, LoKi)", { issueAuthors: { "yetone/magpie#3305": "LoKi" } }), ["LoKi"]);
  assert.deepEqual(names("sessions: Terminal is opened through proc.Command, as every command magpie runs is (spotted by @hezhizhen in #103)"), ["hezhizhen"]);
});

test("commit authors and co-authors are credited, agents and machines not, and no email is given", () => {
  const per = credit([
    { subject: "gui: a group's Save no longer leaves the Routing page blank (#884)", author: "wizzy-yang", email: "244719629+wizzy-yang@users.noreply.github.com", coauthors: [{ name: "magpie-fix", email: "fix@localhost" }] },
    { subject: "gateway: count usage from multiline and final SSE events (#94)", author: "Charan Rathore", email: "charan-rathore@users.noreply.github.com", coauthors: [{ name: "Instinct Agent", email: "agent@instinct.com" }, { name: "Claude Opus 5.5", email: "noreply@anthropic.com" }] },
    { subject: "docker: a server image and a multi-arch build workflow (#245)", author: "Shelken", email: "shelken@example.com", coauthors: [{ name: "glm-5.3-flash", email: "hakase@ooooo.space" }, { name: "yetone", email: "yetoneful@gmail.com" }] },
    { subject: "Merge pull request #1 from someone/branch", author: "ggbdpq", email: "x@y", parents: 2 },
  ]);
  assert.deepEqual(per.map((l) => l.map((p) => p.name)), [["wizzy-yang"], ["Charan Rathore"], ["Shelken"], []]);
  assert.equal(per[0][0].github, "wizzy-yang");
  assert.equal(per[2][0].github, "");
  assert.doesNotMatch(JSON.stringify(per), /@/);
});

test("the exclusion list leaves a person out everywhere", () => {
  const s = "library: an MCP router says which computer it is, so an agent can pick between two (packinggoodsllc_07668 on Discord)";
  assert.deepEqual(names(s, { exclude: ["PackingGoodsLLC_07668"] }), []);
  const commits = [{ subject: s, v: "0.1.2569" }];
  assert.deepEqual(wall(commits, { exclude: ["packinggoodsllc_07668"] }), []);
  assert.deepEqual(excluded(), []); // the checked-in list has comments only
  const dir = mkdtempSync(join(tmpdir(), "credits-"));
  try {
    writeFileSync(join(dir, "x.txt"), "# comment\nPackingGoodsLLC_07668  # asked on Discord\n\n");
    assert.deepEqual(names(s, { exclude: excluded(join(dir, "x.txt")) }), []);
  } finally { rmSync(dir, { recursive: true, force: true }); }
});

test("the release notes' thanks line, in English and Chinese", () => {
  const people = everyone(credit([
    { subject: "library: an MCP router says which computer it is, so an agent can pick between two (packinggoodsllc_07668 on Discord)" },
    { subject: "gateway: a HEIC photo goes to the vendor as JPEG, or PNG with alpha (qikamatata on X)" },
    { subject: "guard: the local masking model reads text a third faster on amd64 (mrwl666)" },
    { subject: "gateway: a HEIC photo is sent once (qikamatata on X)" },
  ]));
  assert.equal(thanks(people, "en"), "Thanks to packinggoodsllc\\_07668, qikamatata and mrwl666, whose reports and code went into this release. Everyone who helped: [contributors](https://usemagpie.ai/docs/contributors).");
  assert.equal(thanks(people, "zh"), "感谢 packinggoodsllc\\_07668、qikamatata 和 mrwl666：这次更新里有你们的反馈与代码。所有贡献者见[贡献墙](https://usemagpie.ai/docs/zh/contributors)。");
  assert.equal(thanks(people.slice(0, 1), "en").startsWith("Thanks to packinggoodsllc\\_07668, whose"), true);
  assert.match(thanks(people.slice(0, 1), "zh"), /里有你的/);
  assert.equal(thanks([], "en"), "");
});

test("the page shows what each person's reports led to, newest first", () => {
  assert.equal(what("redact: a decimal's digits are not masked as a number of their own (willz, Discord)"), "redact: a decimal's digits are not masked as a number of their own");
  assert.equal(what("plugin: gateway key limits are the Key limits plugin on the gateway's budget point (P3)"), "plugin: gateway key limits are the Key limits plugin on the gateway's budget point (P3)");
  const people = wall([
    { subject: "search: searchers picked to follow the one named, tried in their order when it fails (axelburks on Discord)", v: "0.1.2561" },
    { subject: "gui: Busy models last takes a custom threshold, 1 to 9999 (axelburks)", v: "0.1.2564" },
    { subject: "redact: a decimal's digits are not masked as a number of their own (willz, Discord)", v: "0.1.2560" },
  ]);
  assert.deepEqual(people.map((p) => [p.name, p.releases.map((r) => r.v)]), [["axelburks", ["0.1.2564", "0.1.2561"]], ["willz", ["0.1.2560"]]]);
  const html = renderWall({ people: [{ name: "<b>x</b>", via: ["Discord"], github: "x", releases: [{ v: "0.1.1", what: ["a & b"] }] }] }, "zh");
  assert.match(html, /&lt;b&gt;x&lt;\/b&gt;/);
  assert.match(html, /a &amp; b/);
  assert.match(html, /releases\/tag\/v0\.1\.1/);
  assert.match(html, /新的在前/);
  assert.match(renderWall({}, "ja"), /一覧/);
  const many = renderWall({ people: [{ name: "n", releases: [1, 2, 3, 4, 5].map((i) => ({ v: `0.1.${i}`, what: ["w"] })) }] }, "en");
  assert.match(many, /<summary>2 more<\/summary>/);
});

test("issue authors GitHub can't give are left out, not guessed", async () => {
  const refs = [{ repo: "yetone/magpie", n: 1 }, { repo: "yetone/magpie", n: 2 }];
  const ok = async () => ({ ok: true, json: async () => ({ data: { repository: { i1: { author: { login: "alice", __typename: "User" } }, i2: { author: { login: "dependabot", __typename: "Bot" } } } } }) });
  assert.deepEqual(await authors(refs, { fetchImpl: ok, tok: "t" }), { "yetone/magpie#1": "alice", "yetone/magpie#2": "dependabot[bot]" });
  const down = async () => ({ ok: false, status: 502 });
  const err = process.stderr.write;
  process.stderr.write = () => true;
  try {
    assert.deepEqual(await authors(refs, { fetchImpl: down, tok: "t" }), {});
    assert.deepEqual(await authors(refs, { fetchImpl: down, tok: "t", known: { "yetone/magpie#1": "alice" } }), { "yetone/magpie#1": "alice" });
    assert.deepEqual(await authors(refs, { fetchImpl: ok, tok: "" }), {});
  } finally { process.stderr.write = err; }
});
