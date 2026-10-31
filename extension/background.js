// The extension side of chromectl. Chrome starts the native host named below
// when this worker connects, and the open port keeps the worker alive. Every
// message from the host is one request; every request gets exactly one reply
// carrying the same seq.

const HOST = "com.hk9890.chromectl";
const RECONNECT_MS = 2000;

// profileId returns this profile's id, 8 hex digits, made on the first call
// and kept in IndexedDB: every profile has its own, clearing browsing data
// does not touch an extension's, and it needs no permission.
function profileId() {
  return new Promise((resolve, reject) => {
    const open = indexedDB.open("chromectl", 1);
    open.onupgradeneeded = () => open.result.createObjectStore("profile");
    open.onerror = () => reject(open.error);
    open.onsuccess = () => {
      const db = open.result;
      const tx = db.transaction("profile", "readwrite");
      const store = tx.objectStore("profile");
      let id;
      store.get("id").onsuccess = (e) => {
        id = e.target.result;
        if (!id) {
          id = crypto.getRandomValues(new Uint32Array(1))[0].toString(16).padStart(8, "0");
          store.put(id, "id");
        }
      };
      tx.oncomplete = () => {
        db.close();
        resolve(id);
      };
      // A commit can fail with no request error, as on a full quota; only
      // abort fires then, and the host would wait for ever.
      tx.onabort = () => reject(tx.error ?? new Error("the profile id was not saved"));
    };
  });
}

// tabGroups is chrome.tabGroups, which exists only once a reload has granted
// the permission a newer manifest asks for.
function tabGroups() {
  if (!chrome.tabGroups) {
    throw new Error("the chromectl extension lacks the tabGroups permission: reload it at chrome://extensions");
  }
  return chrome.tabGroups;
}

const lastAccessed = (tabs) => Math.max(0, ...tabs.map((t) => t.lastAccessed ?? 0));

function mostRecent(tabs) {
  return tabs.reduce((a, b) => ((b.lastAccessed ?? 0) > (a.lastAccessed ?? 0) ? b : a));
}

// pageOf picks the tab of a group to activate: the one at url, else one under
// url, else any. The boundary keeps github.com/o/repo from taking
// github.com/o/repo-tools for its own page.
function pageOf(tabs, url) {
  const at = tabs.filter((t) => t.url === url);
  const boundary = (rest) => rest !== "" && (url.endsWith("/") || "/?#".includes(rest[0]));
  const under = tabs.filter((t) => t.url?.startsWith(url) && boundary(t.url.slice(url.length)));
  return mostRecent(at.length ? at : under.length ? under : tabs);
}

// raiseGroup raises the group titled title, the one with the most recently
// accessed tab where several share the title, or creates it with url in
// window, else in the last focused normal window, else in a new one.
async function raiseGroup({ title, url, window: windowId }) {
  const groups = await tabGroups().query({});
  const tabs = await chrome.tabs.query({});
  const members = (g) => tabs.filter((t) => t.groupId === g.id);
  const found = groups
    .filter((g) => g.title === title)
    .map((g) => ({ group: g, tabs: members(g) }))
    .filter((g) => g.tabs.length > 0);
  if (found.length > 0) {
    const { group, tabs: inGroup } = found.reduce((a, b) => (lastAccessed(b.tabs) > lastAccessed(a.tabs) ? b : a));
    const tab = pageOf(inGroup, url);
    await tabGroups().update(group.id, { collapsed: false });
    await chrome.tabs.update(tab.id, { active: true });
    await chrome.windows.update(tab.windowId, { focused: true });
    return { id: tab.id, window: tab.windowId, created: false, title: tab.title ?? "" };
  }

  if (!windowId) {
    windowId = await chrome.windows.getLastFocused({ windowTypes: ["normal"] }).then((w) => w.id, () => undefined);
  }
  const tab = windowId
    ? await chrome.tabs.create({ url, windowId, active: true })
    : (await chrome.windows.create({ url, type: "normal", focused: true })).tabs[0];
  const group = await chrome.tabs.group({ tabIds: [tab.id], createProperties: { windowId: tab.windowId } });
  await tabGroups().update(group, { title });
  await chrome.windows.update(tab.windowId, { focused: true });
  return { id: tab.id, window: tab.windowId, created: true, title: tab.title ?? "" };
}

// Two activate-group calls for one title that overlapped would each find no
// group and each create one, so they run one after the other.
let raising = Promise.resolve();

const commands = {
  async identify() {
    return { profile: await profileId() };
  },

  async list() {
    const tabs = await chrome.tabs.query({});
    return tabs.map((t) => ({
      id: t.id,
      window: t.windowId,
      group: t.groupId,
      active: t.active,
      title: t.title ?? "",
      url: t.url ?? "",
    }));
  },

  async activate({ id }) {
    const tab = await chrome.tabs.update(id, { active: true });
    await chrome.windows.update(tab.windowId, { focused: true });
    return { id: tab.id, window: tab.windowId };
  },

  async open({ url, window: windowId }) {
    const tab = await chrome.tabs.create(
      windowId ? { url, windowId, active: true } : { url, active: true },
    );
    await chrome.windows.update(tab.windowId, { focused: true });
    return { id: tab.id, window: tab.windowId };
  },

  async close({ id }) {
    await chrome.tabs.remove(id);
    return {};
  },

  async groups() {
    const groups = await tabGroups().query({});
    const tabs = await chrome.tabs.query({});
    return groups.map((g) => ({
      id: g.id,
      window: g.windowId,
      title: g.title ?? "",
      lastAccessed: lastAccessed(tabs.filter((t) => t.groupId === g.id)),
    }));
  },

  "activate-group"(req) {
    const raised = raising.then(() => raiseGroup(req));
    raising = raised.catch(() => {});
    return raised;
  },
};

function connect() {
  const port = chrome.runtime.connectNative(HOST);
  port.onMessage.addListener(async (req) => {
    if (!Object.hasOwn(commands, req.cmd)) {
      port.postMessage({ seq: req.seq, error: `unknown command ${req.cmd}` });
      return;
    }
    try {
      port.postMessage({ seq: req.seq, result: await commands[req.cmd](req) });
    } catch (e) {
      port.postMessage({ seq: req.seq, error: String(e?.message ?? e) });
    }
  });
  port.onDisconnect.addListener(() => {
    console.log("chromectl host gone:", chrome.runtime.lastError?.message);
    setTimeout(connect, RECONNECT_MS);
  });
}

// Chrome starts an MV3 worker at browser start only for a worker that listens
// for onStartup. The listener does nothing: starting the worker runs connect.
chrome.runtime.onStartup.addListener(() => {});

connect();
