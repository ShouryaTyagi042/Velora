import { createContext, use, useEffect, useState, type ReactNode } from "react";

import { login, logout, me, type User } from "@/api/auth";
import { setToken, setUnauthorizedHandler } from "@/api/client";
import { clearLocal } from "@/progress/store";
import { reconcile } from "@/progress/sync";
import { clearToken, loadToken, saveToken } from "./token";

// loading: reading the Keychain at launch (milliseconds). signedIn's user is null until the
// server confirms the token: offline, the app still opens on a stored token and shows what it
// can, rather than locking you out of a server it merely can't reach.
type State = { status: "loading" } | { status: "signedOut" } | { status: "signedIn"; user: User | null };

type Auth = State & {
  signIn: (username: string, password: string) => Promise<void>;
  signOut: () => Promise<void>;
};

const Context = createContext<Auth | null>(null);

// AuthProvider owns the session for the whole app: the token in the Keychain, the copy the API
// client sends, and the state the root layout guards screens with.
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<State>({ status: "loading" });

  useEffect(() => {
    // Any 401 on a request that carried a token: the session is gone (expired, or logged out on
    // the server). Forget it; the root layout's guard then shows the login screen.
    setUnauthorizedHandler(() => {
      setToken(null);
      void clearToken();
      setState({ status: "signedOut" });
    });

    let cancelled = false;
    void (async () => {
      const token = await loadToken();
      if (cancelled) return;
      if (!token) {
        setState({ status: "signedOut" });
        return;
      }
      setToken(token);
      setState({ status: "signedIn", user: null });
      // Push any progress a previous session saved locally but never got to the server.
      void reconcile();
      try {
        const user = await me();
        if (!cancelled) setState({ status: "signedIn", user });
      } catch {
        // A 401 has already signed us out (the handler above). Anything else is the network:
        // keep the stored token and let screens show "can't reach the server".
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const signIn = async (username: string, password: string) => {
    const { token, user } = await login(username, password); // throws ApiError: the screen shows it
    await saveToken(token);
    setToken(token);
    setState({ status: "signedIn", user });
    void reconcile();
  };

  const signOut = async () => {
    await reconcile(); // last chance for unsynced saves; they're cleared below
    await logout().catch(() => {}); // offline: the session expires on its own within 30 days
    setToken(null);
    await Promise.all([clearToken(), clearLocal()]);
    setState({ status: "signedOut" });
  };

  return <Context value={{ ...state, signIn, signOut }}>{children}</Context>;
}

export function useAuth(): Auth {
  const a = use(Context);
  if (!a) throw new Error("useAuth needs an AuthProvider above it");
  return a;
}
