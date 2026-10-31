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
      tx.onerror = () => reject(tx.error);
    };
  });
}

const commands = {
  async identify() {
    return { profile: await profileId() };
  },

  async list() {
    const tabs = await chrome.tabs.query({});
    return tabs.map((t) => ({
      id: t.id,
      window: t.windowId,
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
