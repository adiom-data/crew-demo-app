import { useCallback, useEffect, useState } from "react";
import type { GetSessionResponse, User } from "./gen/sample/v1/sample_pb";
import { authTokenManager, sampleClient } from "./api/clients";

export type SessionState =
  | { status: "loading" }
  | { status: "anonymous" }
  | { status: "authenticated"; session: GetSessionResponse; user?: User }
  | { status: "error"; message: string };

// useSession lifts the auth/session flow that used to live in App.tsx: it mints
// an app token, verifies it via GetSession, and exposes refresh/logout.
export function useSession() {
  const [state, setState] = useState<SessionState>({ status: "loading" });

  const refresh = useCallback(async () => {
    setState({ status: "loading" });
    const token = await authTokenManager.getToken();
    if (!token) {
      setState({ status: "anonymous" });
      return;
    }
    try {
      const session = await sampleClient.getSession({});
      setState({ status: "authenticated", session, user: session.user });
    } catch (error) {
      setState({ status: "error", message: errorMessage(error) });
    }
  }, []);

  const logout = useCallback(async () => {
    await authTokenManager.logout();
    setState({ status: "anonymous" });
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  return { state, refresh, logout };
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Something went wrong.";
}
