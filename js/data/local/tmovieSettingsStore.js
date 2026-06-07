import { createProfileScopedStore } from "./profileScopedStore.js";

const KEY = "tmovieSettings";

export const TMOVIE_SETTINGS_DEFAULTS = {
  enabled: false,
  serverUrl: "",
  apiToken: "",
  paired: false
};

function normalizeServerUrl(value) {
  const url = String(value || "").trim();
  if (!url) {
    return "";
  }
  return url.replace(/\/+$/, "");
}

function normalizeTMovieSettings(value = {}) {
  const source = value && typeof value === "object" ? value : {};
  return {
    ...TMOVIE_SETTINGS_DEFAULTS,
    enabled: Boolean(source.enabled),
    serverUrl: normalizeServerUrl(source.serverUrl),
    apiToken: String(source.apiToken || "").trim(),
    paired: Boolean(source.paired)
  };
}

const store = createProfileScopedStore({
  key: KEY,
  normalize: normalizeTMovieSettings
});

export const TMovieSettingsStore = {

  getForProfile(profileId) {
    return store.getForProfile(profileId);
  },

  get() {
    return store.get();
  },

  replaceForProfile(profileId, nextValue, options = {}) {
    return store.replaceForProfile(profileId, nextValue, options);
  },

  setForProfile(profileId, partial, options = {}) {
    return store.setForProfile(profileId, partial, options);
  },

  set(partial, options = {}) {
    return store.set(partial, options);
  }

};
