import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

// The session token lives in the Keychain (iOS) / Keystore (Android): encrypted, and only this
// app can read it. Never AsyncStorage, which is a plain file anyone with the backup can read.
// Expo web (for development) has no secure store, so there the token is kept in memory only and
// a reload means signing in again: better than putting it in localStorage.
const KEY = "velora.session";
const persistent = Platform.OS !== "web";

export async function loadToken(): Promise<string | null> {
  if (!persistent) return null;
  try {
    return await SecureStore.getItemAsync(KEY);
  } catch {
    return null; // unreadable (e.g. restored onto a new device): sign in again
  }
}

export async function saveToken(token: string): Promise<void> {
  if (persistent) await SecureStore.setItemAsync(KEY, token);
}

export async function clearToken(): Promise<void> {
  if (!persistent) return;
  try {
    await SecureStore.deleteItemAsync(KEY);
  } catch {
    // Nothing stored, or the store is unavailable: either way there's no token now.
  }
}
