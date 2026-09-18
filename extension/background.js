// The extension side of chromectl. Chrome starts the native host named below
// when this worker connects, and the open port keeps the worker alive. Every
// message from the host is one request; every request gets exactly one reply
// carrying the same seq.

const HOST = "com.hk9890.chromectl";
const RECONNECT_MS = 2000;

const commands = {
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
    const run = commands[req.cmd];
    if (!run) {
      port.postMessage({ seq: req.seq, error: `unknown command ${req.cmd}` });
      return;
    }
    try {
      port.postMessage({ seq: req.seq, result: await run(req) });
    } catch (e) {
      port.postMessage({ seq: req.seq, error: String(e?.message ?? e) });
    }
  });
  port.onDisconnect.addListener(() => {
    console.log("chromectl host gone:", chrome.runtime.lastError?.message);
    setTimeout(connect, RECONNECT_MS);
  });
}

connect();
