const LAST_WINDOW_ALARM = "kotik-last-browser-window-closed";
const LAST_WINDOW_FALLBACK_MS = 30_000;

export async function disconnectWhenNoBrowserWindows() {
  const windows = await chrome.windows
    .getAll({ windowTypes: ["normal"] })
    .catch(() => []);

  if (windows.length !== 0) {
    return false;
  }

  const disableEngine = globalThis.__kotikDisableEngine;
  if (typeof disableEngine !== "function") {
    console.warn("[kotik] direct engine shutdown hook is unavailable");
    return false;
  }

  const response = await Promise.resolve(disableEngine()).catch((error) => {
      console.warn("[kotik] disconnect on browser close failed:", error);
      return null;
  });

  return response?.ok === true;
}

chrome.windows.onRemoved.addListener(() => {
  // onRemoved fires after the window is gone.  Do the safety-critical stop
  // immediately: sub-minute chrome.alarms are not guaranteed to run on time.
  void disconnectWhenNoBrowserWindows();

  // A delayed re-check is only a fallback for an interrupted service worker.
  chrome.alarms.create(LAST_WINDOW_ALARM, {
    when: Date.now() + LAST_WINDOW_FALLBACK_MS,
  });
});

chrome.alarms.onAlarm.addListener((alarm) => {
  if (alarm.name === LAST_WINDOW_ALARM) {
    void disconnectWhenNoBrowserWindows();
  }
});
